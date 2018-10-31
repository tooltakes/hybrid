package hybrid

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/empirefox/hybrid/hybriddomain"
)

var (
	ErrRequestURI = errors.New("bad RequestURI")
)

type ContextConfig struct {
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
	if req.URL.Scheme == "" {
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
		Connect:      req.Method == "CONNECT",
		HasPort:      true,
	}

	if !c.Connect {
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

	c.HostPort, c.HostNoPort, c.Port = authorityAddrFull(req.URL.Scheme, req.Host)
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

func (c *Context) CloneRequest() *http.Request {
	req := c.Request
	ctx := req.Context()
	outreq := req.WithContext(ctx) // includes shallow copies of maps, but okay
	outreq.Header = cloneHeader(req.Header)
	return outreq
}

// SendRequest write Request to remote, nothing more.
func (c *Context) SendRequest(remote net.Conn, withProxy bool) (err error) {
	req := c.Request
	if req.Body == http.NoBody {
		// if not nil, req.Write will not return
		req.Body = nil
	} else {
		req.Body = ioutil.NopCloser(req.Body)
	}
	if withProxy {
		err = req.WriteProxy(remote)
	} else {
		err = req.Write(remote)
	}
	return
}

// ProxyUp dial to exist http server, SendRequest to remote, waits for remote close.
// Used by final node.
func (c *Context) ProxyUp(proxyaddr string, keepAlive bool) error {
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

	err = c.SendRequest(remote, true)
	if err != nil {
		return err
	}

	c.PipeConn(remote)
	return nil
}

// Direct dial to final target, SendRequest to remote, waits for remote close.
// Used by final node.
func (c *Context) Direct() error {
	remote, err := net.Dial("tcp", c.DialHostPort)
	if err != nil {
		return err
	}
	defer remote.Close()

	req := c.Request
	if c.Connect {
		c.Writer.Write(StandardConnectOK)
	} else {
		// must remove connection header
		if req.Header != nil {
			req.Header = make(http.Header)
		}
		delete(req.Header, "Proxy-Connection")
		req.Header.Set("Connection", "close")
	}

	// TODO modify stream to set Connection: close
	err = c.SendRequest(remote, false)
	if err != nil {
		return err
	}

	c.PipeConn(remote)
	return nil
}

// PipeRoundTrip requests with rp, waits for pipe end.
func (c *Context) PipeRoundTrip(rp func(*http.Request) (*http.Response, error)) error {
	req := c.Request
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)
	res, err := rp(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return c.copyFromResponse(res)
}

func (c *Context) copyFromResponse(res *http.Response) error {
	if c.Connect {
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("Response %d", res.StatusCode)
		}
		if c.Writer != nil {
			c.Writer.Write(StandardConnectOK)
			c.Copy(c.Writer, res.Body)
		} else {
			c.ResponseWriter.WriteHeader(http.StatusOK)
			flusher, ok := c.ResponseWriter.(http.Flusher)
			if ok {
				flusher.Flush()
			}
			c.Copy(c.ResponseWriter, res.Body)
		}
	} else {
		res.Close = true
		if c.Writer != nil {
			res.Write(c.Writer)
		} else {
			res.Write(c.ResponseWriter)
		}
	}
	return nil
}

// PipeConn waits for remote close. Used by final node. Must close remote after.
func (c *Context) PipeConn(remote net.Conn) {
	go c.copyToRemote(remote)

	if c.Writer != nil {
		c.Copy(c.Writer, remote)
	} else {
		c.Copy(c.ResponseWriter, remote)
	}
}

func (c *Context) copyToRemote(remote io.WriteCloser) {
	r := c.Request.Body
	if r == nil || r == http.NoBody {
		r = c.UnsafeReader
	}

	c.Copy(remote, r)
	remote.Close()
}

// Copy copies src to dst with BufferPool. Enable timeout read if src is net.Conn.
func (c *Context) Copy(dst io.Writer, src io.Reader) (int64, error) {
	buf := c.BufferPool.Get()
	defer c.BufferPool.Put(buf)
	dr, ok := src.(SetReadDeadlineReader)
	if ok {
		return NewTimeoutWriterTo(dr).WriteTo(dst, buf, c.TimeoutForCopy)
	}
	return io.CopyBuffer(dst, src, buf)
}

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
