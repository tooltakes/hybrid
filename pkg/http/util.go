package hybridhttp

import (
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

func SimpleListenAndServe(listener net.Listener, serveConn func(c net.Conn)) error {
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		c, e := listener.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0
		go serveConn(c)
	}
}

// AuthorityAddrFull returns a given authority (a host/IP, or host:port / ip:port)
// and returns a host:port. The port 443 is added if needed.
// imported from http2/transport.go
func AuthorityAddrFull(scheme string, authority string) (hostport, hostnoport, port string) {
	host, port, err := net.SplitHostPort(authority)
	if err != nil { // authority didn't have a port
		port = "443"
		if scheme == "ws" || scheme == "http" {
			port = "80"
		}
		host = authority
	}
	if a, err := idna.ToASCII(host); err == nil {
		host = a
	}
	// IPv6 address literal, without a port:
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host + ":" + port, host, port
	}
	return net.JoinHostPort(host, port), host, port
}

func CloneRequest(req *http.Request) *http.Request {
	ctx := req.Context()
	outreq := req.WithContext(ctx) // includes shallow copies of maps, but okay
	outreq.Header = CloneHeader(req.Header)
	return outreq
}

func CloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}
