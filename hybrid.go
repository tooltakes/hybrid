package hybrid

import (
	"net"
	"net/http"
)

type Hybrid struct {
	Routers      []Router
	Proxies      map[string]Proxy
	LocalServers map[string]http.Handler
	LocalHandler http.Handler
}

func (h *Hybrid) Proxy(conn net.Conn) {
	defer conn.Close()

	c, err := ReadContext(conn)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 400 Bad Request Connect\r\n\r\n"))
		return
	}

	if c.Hybrid != "" {
		if h.LocalServers != nil {
			handler, ok := h.LocalServers[c.Hybrid]
			if ok {
				handler.ServeHTTP(NewResponseWriter(c.Writer), c.Request)
				return
			}
		}

		if IsHybridLocal(c.Hybrid) {
			if h.LocalHandler != nil {
				h.LocalHandler.ServeHTTP(NewResponseWriter(c.Writer), c.Request)
			} else {
				conn.Write(StandardLocalServiceUnaviliable)
			}
			return
		}

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
