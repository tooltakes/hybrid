package hybridcore

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/empirefox/hybrid/pkg/domain"
	"github.com/empirefox/hybrid/pkg/http"
)

var (
	// StandardConnectOK indicates Content-Length: -1
	StandardConnectOK = []byte("HTTP/1.1 200 OK\r\n\r\n")

	ErrRequestURI = errors.New("bad RequestURI")
)

type ContextConfig struct {
	Transport      http.RoundTripper
	BufferPool     httputil.BufferPool
	TimeoutForCopy time.Duration
}

type Context struct {
	ContextConfig

	ResponseWriter http.ResponseWriter
	Writer         io.Writer

	Request      *http.Request
	UnsafeReader io.ReadCloser // only used when Body is nil or NoBody
	Connect      bool
	HostPort     string // maybe empty
	HostNoPort   string
	Port         string
	HasPort      bool
	IP           net.IP
	DialHostPort string
	Domain       hybriddomain.Domain

	nopCloser io.ReadCloser
}

func NewContextWithConn(cc *ContextConfig, conn net.Conn) (*Context, error) {
	bufConn := bufio.NewReader(conn)
	req, err := http.ReadRequest(bufConn)
	if err != nil {
		return nil, err
	}

	return newContext(cc, req, nil, conn)
}

func NewContextFromHandler(cc *ContextConfig, w http.ResponseWriter, req *http.Request) (*Context, error) {
	return newContext(cc, req, w, nil)
}

func newContext(cc *ContextConfig, req *http.Request, resp http.ResponseWriter, conn net.Conn) (*Context, error) {
	isConnect := req.Method == "CONNECT"
	if !isConnect && req.URL.Scheme == "" {
		// parsing CONNECT removed schema
		return nil, ErrRequestURI
	}

	if conn == nil {
		hijacker, ok := resp.(http.Hijacker)
		if ok {
			conn, _, _ = hijacker.Hijack()
		}
	}

	// conn's first
	// TODO is it right?
	var unsafeReader io.ReadCloser = conn
	if unsafeReader == nil {
		unsafeReader = req.Body
	}

	c := Context{
		ContextConfig: *cc,

		ResponseWriter: resp,
		Writer:         conn,

		Request:      req,
		UnsafeReader: unsafeReader,
		Connect:      isConnect,
		HasPort:      true,
	}

	if c.Connect {
		req.ContentLength = -1
	} else {
		// Do not keep-alive any reverse request.
		// Server will auto add Connection: close?
		// Every response add too?
		for k := range req.Header {
			if strings.EqualFold(k, "upgrade") {
				c.Connect = true
				break
			}
		}
	}
	req.RequestURI = ""

	if req.Host == "" {
		// TODO enable this:
		//	GET /index.html HTTP/1.1
		return nil, ErrRequestURI
	}

	err := c.parseHostPortIP()
	if err != nil {
		return nil, err
	}

	err = c.parseDomain()
	if err != nil {
		return nil, err
	}

	c.adjustRequestAndDialHost()

	return &c, nil
}

func (c *Context) parseHostPortIP() error {
	req := c.Request
	if c.Connect {
		// requestURI must be a valid authority

		// ReadRequest already do:
		// RFC 2616: Must treat
		//	GET /index.html HTTP/1.1
		//	Host: www.google.com
		// and
		//	GET http://www.google.com/index.html HTTP/1.1
		//	Host: doesntmatter
		// the same. In the second case, any Host line is ignored.

		// In above two cases, Host is always non-empty, but
		//	GET /index.html HTTP/1.1
		// both Host and URL.Host is empty.
		hostNoPort, port, err := net.SplitHostPort(req.Host)
		if err != nil {
			return err
		}

		c.HostNoPort = hostNoPort
		c.Port = port
		c.HasPort = true
		c.HostPort = net.JoinHostPort(hostNoPort, port)
		c.IP = net.ParseIP(c.HostNoPort)

		return nil
	}

	c.HostPort, c.HostNoPort, c.Port = hybridhttp.AuthorityAddrFull(req.URL.Scheme, req.Host)
	c.HasPort = c.Port != ""
	c.IP = net.ParseIP(c.HostNoPort)
	return nil
}

func (c *Context) parseDomain() error {
	domain, err := hybriddomain.NewDomain(c.HostNoPort)
	if err != nil {
		return err
	}

	c.Domain = *domain
	return nil
}

func (c *Context) adjustRequestAndDialHost() {
	req := c.Request
	if c.Domain.IsHybrid {
		// For client requests Host optionally overrides the Host
		// header to send. If empty, the Request.Write method uses
		// the value of URL.Host. Host may contain an international
		// domain name.
		req.Host = c.Domain.NextHostname
		if c.HasPort {
			req.Host = req.Host + ":" + c.Port
		}
		req.URL.Host = req.Host

		c.DialHostPort = c.Domain.DialHostname + ":" + c.Port
	} else {
		c.DialHostPort = c.HostPort
	}
}

// SendRequest write Request to remote, nothing more.
func (c *Context) SendRequest(remote net.Conn, withProxy bool) (err error) {
	req := c.Request
	if req.Body != http.NoBody {
		req.Body = c.NopCloserBody()
	}
	if withProxy {
		err = req.WriteProxy(remote)
	} else {
		err = req.Write(remote)
	}
	if err != nil {
		if _, ok := err.(hybridhttp.FromWriterError); !ok {
			// err from Request.Body
			remote.Close()
		}
		return
	}

	c.copyToRemote(remote)
	return
}

// ProxyUp dial to exist http server, SendRequest to remote, waits for remote close.
// Used by final node.
func (c *Context) ProxyUp(proxyaddr string, tp http.RoundTripper, keepAlive bool) error {
	if !c.Connect && c.Writer == nil {
		// reverse to c.ResponseWriter
		c.ReverseToResponse(tp)
		return nil
	}

	remote, err := net.Dial("tcp", proxyaddr)
	if err != nil {
		return err
	}
	defer remote.Close()

	req := c.Request
	if keepAlive {
		req.Header.Del("Connection")
	} else {
		req.Header.Set("Connection", "close")
	}

	go c.SendRequest(remote, true)
	if c.Connect && c.Writer == nil {
		// connect to c.ResponseWriter
		bw := bufio.NewReader(remote)
		res, err := http.ReadResponse(bw, req)
		if err != nil {
			return err
		}
		res.Body = http.NoBody

		if res.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(res.Body)
			return fmt.Errorf("Response %d, err: %s", res.StatusCode, body)
		}

		c.writeConncectOK()
		if n := bw.Buffered(); n > 0 {
			// TODO will this happen?
			b, err := bw.Peek(n)
			if err != nil {
				return err
			}
			_, err = c.writeBytesFromRemote(b)
			if err != nil {
				return err
			}
		}
	}

	c.copyFromRemote(remote)
	return nil
}

func (c *Context) writeBytesFromRemote(b []byte) (int, error) {
	if c.Writer != nil {
		return c.Writer.Write(b)
	}
	return c.ResponseWriter.Write(b)
}

// Direct dial to final target, SendRequest to remote, waits for remote close.
// Used by final node.
func (c *Context) Direct() error {
	if !c.Connect && c.Writer == nil {
		// reverse to c.ResponseWriter
		c.ReverseToResponse(c.Transport)
		return nil
	}

	remote, err := net.Dial("tcp", c.DialHostPort)
	if err != nil {
		return err
	}
	defer remote.Close()

	req := c.Request
	if c.Connect {
		c.writeConncectOK()
		go c.copyToRemote(remote)
		c.copyFromRemote(remote)
		return nil
	}

	// reverse to c.Writer
	// must remove connection header
	if req.Header != nil {
		req.Header = make(http.Header)
	}
	delete(req.Header, "Proxy-Connection")
	req.Header.Set("Connection", "close")
	// TODO modify stream to set Connection: close
	go c.SendRequest(remote, false)
	c.copyFromRemote(remote)
	return nil
}

// PipeTransport requests with rp, waits for pipe end.
func (c *Context) PipeTransport(tp http.RoundTripper) error {
	req := c.Request

	if !c.Connect && c.Writer == nil {
		// reverse to c.ResponseWriter
		c.ReverseToResponse(tp)
		return nil
	}

	res, err := tp.RoundTrip(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if !c.Connect {
		// reverse to c.Writer
		res.Close = true
		res.Write(c.Writer)
		return nil
	}

	// CONNECT below
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("Response %d, err: %s", res.StatusCode, body)
	}

	c.writeConncectOK()
	c.copyFromRemote(res.Body)
	return nil
}

func (c *Context) ReverseToResponse(tp http.RoundTripper) {
	rp := httputil.ReverseProxy{
		Director:       func(r *http.Request) {},
		Transport:      tp,
		FlushInterval:  c.TimeoutForCopy,
		BufferPool:     c.BufferPool,
		ModifyResponse: nil,
	}
	rp.ServeHTTP(c.ResponseWriter, c.Request)
}

func (c *Context) writeConncectOK() {
	if c.Writer != nil {
		c.Writer.Write(StandardConnectOK)
	} else {
		c.ResponseWriter.WriteHeader(http.StatusOK)
		flusher, ok := c.ResponseWriter.(http.Flusher)
		if ok {
			flusher.Flush()
		}
	}
}

func (c *Context) copyToRemote(remote io.WriteCloser) {
	c.Copy(remote, c.bodyReader())
	remote.Close()
}

func (c *Context) copyFromRemote(remote io.Reader) {
	if c.Writer != nil {
		c.Copy(c.Writer, remote)
	} else {
		c.Copy(c.ResponseWriter, remote)
	}
}

func (c *Context) NopCloserBody() io.ReadCloser {
	if c.nopCloser == nil {
		c.nopCloser = ioutil.NopCloser(c.bodyReader())
	}
	return c.nopCloser
}

func (c *Context) bodyReader() io.ReadCloser {
	r := c.Request.Body
	if r == nil || r == http.NoBody {
		r = c.UnsafeReader
	}
	return r
}

// Copy copies src to dst with BufferPool. Enable timeout read if src is net.Conn.
func (c *Context) Copy(dst io.Writer, src io.Reader) (int64, error) {
	buf := c.BufferPool.Get()
	defer c.BufferPool.Put(buf)
	dr, ok := src.(hybridhttp.SetReadDeadlineReadCloser)
	if ok {
		return hybridhttp.NewTimeoutWriterTo(dr, buf, c.TimeoutForCopy).WriteTo(dst)
	}
	return io.CopyBuffer(dst, src, buf)
}

// HybridHttpErr global level HttpErr.
func (c *Context) HybridHttpErr(code int, info string) {
	he := &HttpErr{
		Code:       code,
		ClientType: "Hybrid",
		ClientName: "",
		TargetHost: c.HostPort,
		Info:       info,
	}
	c.HttpErr(he)
}

func (c *Context) HttpErr(he *HttpErr) {
	if c.Writer != nil {
		he.Write(c.Writer)
	} else {
		he.WriteResponse(c.ResponseWriter)
	}
}

func (c *Context) proxy(p Proxy) {
	err := p.Do(c)
	if err != nil {
		p.HttpErr(c, http.StatusBadGateway, err.Error())
	}
}
