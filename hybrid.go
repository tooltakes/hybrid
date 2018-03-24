package hybrid

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

type Hybrid struct {
	Routers      []Router
	Proxies      map[string]Proxy
	LocalHandler http.Handler
}

func (h *Hybrid) Proxy(conn net.Conn) {
	defer conn.Close()

	c, err := ReadContext(conn)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 400 Bad Request Connect\r\n\r\n"))
		return
	}

	switch c.Hybrid {
	case HostLocal0, HostLocalhost, HostLocal127, HostLocal0000:
		if h.LocalHandler != nil {
			h.LocalHandler.ServeHTTP(NewResponseWriter(c.Writer), c.Request)
		} else {
			conn.Write(StandardLocalServiceUnaviliable)
		}
		return
	}

	// XXX.hybrid
	if c.Hybrid != "" {
		p, ok := h.Proxies[c.Hybrid]
		if !ok {
			conn.Write([]byte("HTTP/1.1 404 Hybrid Not Found\r\n\r\n"))
			return
		}

		// http://abcb.hybrid:80/192.168.22.22/a.html => http://192.168.22.22:80/a.html from server
		req := c.Request
		path := req.URL.Path
		if path[0] == '/' {
			path = path[1:]
		}
		s := strings.IndexByte(path, '/')
		// req.Host => authority|target
		switch path[:s] {
		case HostLocal0, HostLocalhost, HostLocal127, HostLocal0000:
			if c.Port == "0" {
				req.Host = HostLocalServer
			} else {
				req.Host = fmt.Sprintf("%s:%s", HostLocalServer, c.Port)
			}
		default:
			req.Host = fmt.Sprintf("%s:%s", path[:s], c.Port)
		}
		req.URL.Path = path[s:]
		p.Do(c)
		return
	}

	for _, rc := range h.Routers {
		if rc.Disabled() {
			continue
		}

		// //NEXT //DIRECT tcp://127.0.0.1:9999, tox://area1,
		// ?http://127.0.0.1:8899, ?file://./ant
		proxy := rc.Route(c)
		if proxy == nil {
			continue
		}

		proxy.Do(c)
		return
	}
	conn.Write([]byte("HTTP/1.1 502 No Proxy Router\r\n\r\n"))
}
