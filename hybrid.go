package hybrid

import (
	"net/http"

	"go.uber.org/zap"
)

type Hybrid struct {
	Log           *zap.Logger
	ContextConfig *ContextConfig
	Routers       []Router
	Proxies       map[string]Proxy
	LocalServers  map[string]http.Handler
}

func (h *Hybrid) Proxy(c *Context, err error) {
	if err != nil {
		c.HybridHttpErr(http.StatusBadRequest, err.Error())
		return
	}

	if c.Domain.IsHybrid {
		if c.Domain.IsEnd {
			// xxx.over.-a.hybrid, xxx.with.hybrid, xxx.over.hybrid
			// DialHostname=xxx
			if h.LocalServers != nil {
				handler, ok := h.LocalServers[c.Domain.DialHostname]
				if ok {
					handler.ServeHTTP(NewResponseWriter(c.Writer), c.Request)
					return
				}
			}
			// dial: c.DialHostPort
			h.proxy(c, DirectProxy)
			return
		}

		p, ok := h.Proxies[c.Domain.Next]
		if !ok {
			c.HybridHttpErr(http.StatusNotFound, c.Domain.Next)
			return
		}
		h.proxy(c, p)
		return
	}

	for _, rc := range h.Routers {
		if rc.Disabled() {
			continue
		}

		p := rc.Route(c)
		if p == nil {
			continue
		}

		h.proxy(c, p)
		return
	}
	c.HybridHttpErr(http.StatusNotFound, "")
}

func (h *Hybrid) proxy(c *Context, p Proxy) {
	err := p.Do(c)
	if err != nil {
		p.HttpErr(c, http.StatusBadGateway, err.Error())
	}
}
