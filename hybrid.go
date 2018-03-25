package hybrid

import (
	"net"
	"net/http"
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
		p.Do(c)
		return
	}

	for _, rc := range h.Routers {
		if rc.Disabled() {
			continue
		}

		proxy := rc.Route(c)
		if proxy == nil {
			continue
		}

		proxy.Do(c)
		return
	}
	conn.Write([]byte("HTTP/1.1 502 No Proxy Router\r\n\r\n"))
}
