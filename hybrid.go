package hybrid

import (
	"net"
)

type Hybrid struct {
	Routers []RouteClient
}

func (h *Hybrid) Proxy(conn net.Conn) {
	defer conn.Close()

	c, err := ReadContext(conn)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 400 Bad Request Connect\r\n\r\n"))
		return
	}

	for _, rc := range h.Routers {
		if rc.Disabled() {
			continue
		}

		// //NEXT //DIRECT tcp://127.0.0.1:9999, tox://area1,
		// ?http://127.0.0.1:8899, ?file://./ant
		proxy := rc.Route(c)
		if proxy == nil || proxy.Host == "NEXT" {
			continue
		}

		if proxy.Host == "DIRECT" {
			c.Direct()
			return
		}

		if exist := rc.Exist(); exist != "" {
			c.ProxyUp(exist)
		} else {
			rc.Proxy(c, proxy)
		}
		return
	}
	conn.Write([]byte("HTTP/1.1 502 No Trip\r\n\r\n"))
}
