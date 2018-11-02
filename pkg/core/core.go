package hybridcore

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/empirefox/hybrid/pkg/http"
)

type Core struct {
	Log           *zap.Logger
	ContextConfig *ContextConfig
	Routers       []Router
	Proxies       map[string]Proxy
	LocalServers  map[string]http.Handler
}

func (core *Core) Proxy(c *Context) {
	if c.Domain.IsHybrid {
		if c.Domain.IsEnd {
			// xxx.over.-a.hybrid, xxx.with.hybrid, xxx.over.hybrid
			// DialHostname=xxx
			if core.LocalServers != nil {
				handler, ok := core.LocalServers[c.Domain.DialHostname]
				if ok {
					handler.ServeHTTP(hybridhttp.NewResponseWriter(c.Writer), c.Request)
					return
				}
			}
			// dial: c.DialHostPort
			c.proxy(DirectProxy)
			return
		}

		p, ok := core.Proxies[c.Domain.Next]
		if !ok {
			c.HybridHttpErr(http.StatusNotFound, c.Domain.Next)
			return
		}
		c.proxy(p)
		return
	}

	for _, rc := range core.Routers {
		if rc.Disabled() {
			continue
		}

		p := rc.Route(c)
		if p == nil {
			continue
		}

		c.proxy(p)
		return
	}
	c.HybridHttpErr(http.StatusNotFound, "")
}
