package hybridcore

import (
	"context"
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
	if c.ResponseWriter != nil {
		req := c.Request
		ctx := req.Context()
		if cn, ok := c.ResponseWriter.(http.CloseNotifier); ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			notifyChan := cn.CloseNotify()
			go func() {
				select {
				case <-notifyChan:
					cancel()
				case <-ctx.Done():
				}
			}()
		}
		req.WithContext(ctx)
	}

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
			if c.Domain.IsOver {
				core.routeProxy(c)
				return
			}

			c.HybridHttpErr(http.StatusNotFound, "")
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

	core.routeProxy(c)
}

func (core *Core) routeProxy(c *Context) {
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

	c.proxy(DirectProxy)
}
