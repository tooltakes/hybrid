package hybrid

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

var (
	ErrRequestURI = errors.New("RequestURI Failed")
)

type Context struct {
	Request      *http.Request
	Writer       io.Writer
	UnsafeReader io.ReadCloser
	Connect      bool
	HostPort     string
	HostNoPort   string
	Port         string
	HasPort      bool
	IP           net.IP
	Hybrid       string
}

func ReadContext(conn net.Conn) (*Context, error) {
	bufConn := bufio.NewReader(conn)
	req, err := http.ReadRequest(bufConn)
	if err != nil {
		return nil, err
	}

	c := Context{
		Request:      req,
		Writer:       conn,
		UnsafeReader: conn,
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

	c.parseHybrid()
	err = c.parseHostPortIP()
	if err != nil {
		return nil, err
	}

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
		hostNoPort, port, err := net.SplitHostPort(req.Host)
		if err != nil {
			return err
		}

		c.HostNoPort = hostNoPort
		c.Port = port
		c.HostPort = net.JoinHostPort(hostNoPort, port)
		c.IP = net.ParseIP(c.HostNoPort)

		return nil
	}

	if req.URL.Scheme == "" || req.URL.Host == "" {
		return ErrRequestURI
	}

	c.HostPort, c.HostNoPort, c.Port = authorityAddrFull(req.URL.Scheme, req.URL.Host)
	c.IP = net.ParseIP(c.HostNoPort)
	return nil
}

func (c *Context) parseHybrid() {
	req := c.Request
	hostNoPort, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		c.HasPort = false
		hostNoPort = req.Host
	}
	if strings.HasSuffix(hostNoPort, HostHybridSuffix) {
		// abcd.hybrid:880 => the server program
		// 0.abcd.hybrid:80 => 0.0.0.0.abcd.hybrid:80 => the port 80 on server
		// 192.168.22.22.abcd.hybrid:80/a.html => 192.168.22.22:80/a.html from server
		c.Hybrid = strings.TrimSuffix(hostNoPort, HostHybridSuffix)
		idx := strings.LastIndexByte(c.Hybrid, '.')
		if idx == -1 {
			// abcd.hybrid:80
			req.Host = ""
		} else {
			// 0.abcd.hybrid:80
			req.Host = c.Hybrid[:idx]
			c.Hybrid = c.Hybrid[idx+1:]
			if req.Host == HostLocal0 {
				req.Host = HostLocal0000
			}
		}
		if c.HasPort {
			if req.Host == "" {
				req.Host = HostLocalServer
			} else {
				req.Host = req.Host + ":" + port
			}
		}
		req.URL.Host = req.Host
	}
}

func (c *Context) CloneRequest() *http.Request {
	req := c.Request
	ctx := req.Context()
	outreq := req.WithContext(ctx) // includes shallow copies of maps, but okay
	outreq.Header = cloneHeader(req.Header)
	return outreq
}

func (c *Context) ProxyUp(proxyaddr string, keepAlive bool) {
	req := c.Request
	if keepAlive {
		req.Header.Del("Connection")
	} else {
		req.Header.Set("Connection", "close")
	}

	remote, err := net.Dial("tcp", proxyaddr)
	if err != nil {
		c.Writer.Write([]byte("HTTP/1.1 502 ProxyUp Dail Fail\r\n\r\n"))
		return
	}
	defer remote.Close()

	if req.Body == http.NoBody {
		// if not nil, req.Write will not return
		req.Body = nil
	} else {
		req.Body = ioutil.NopCloser(req.Body)
	}
	err = req.WriteProxy(remote)
	if err != nil {
		c.Writer.Write([]byte("HTTP/1.1 502 ProxyUp Fail\r\n\r\n"))
		return
	}
	if req.Body == nil {
		req.Body = ioutil.NopCloser(c.UnsafeReader)
	}

	go func() {
		io.Copy(remote, req.Body)
		remote.Close()
	}()
	io.Copy(c.Writer, remote)
}

func (c *Context) Direct() {
	remote, err := net.Dial("tcp", c.HostPort)
	if err != nil {
		c.Writer.Write([]byte("HTTP/1.1 404 Direct Dail Timeout\r\n\r\n"))
		return
	}
	defer remote.Close()

	req := c.Request
	if c.Connect {
		c.Writer.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		go func() {
			// NoBody for CONNECT
			io.Copy(remote, c.UnsafeReader)
			remote.Close()
		}()
		io.Copy(c.Writer, remote)
		return
	}

	// must remove connection header
	if req.Header != nil {
		req.Header = make(http.Header)
	}
	delete(req.Header, "Proxy-Connection")
	req.Header.Set("Connection", "close")

	if req.Body == http.NoBody {
		// if not nil, req.Write will not return
		req.Body = nil
	} else {
		req.Body = ioutil.NopCloser(req.Body)
	}
	err = req.Write(remote)
	if err != nil {
		c.Writer.Write([]byte("HTTP/1.1 502 Direct Remote Fail\r\n\r\n"))
		return
	}
	if req.Body == nil {
		req.Body = ioutil.NopCloser(c.UnsafeReader)
	}
	go func() {
		io.Copy(remote, req.Body)
		remote.Close()
	}()
	// TODO modify stream to set Connection: close
	io.Copy(c.Writer, remote)
	return
}
