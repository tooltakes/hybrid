package hybrid

import (
	"net"
	"net/url"
)

type NetRouterConfig struct {
	Disabled bool
	Exist    string

	IPs  map[*net.IP]*url.URL
	Nets map[*net.IPNet]*url.URL

	Unmatched *url.URL
}

type NetRouter struct {
	Config *NetRouterConfig
}

func (r *NetRouter) Disabled() bool { return r.Config.Disabled }

func (r *NetRouter) Exist() string { return r.Config.Exist }

func (r *NetRouter) Route(c *Context) (proxy *url.URL) {
	for i, u := range r.Config.IPs {
		if i.Equal(c.IP) {
			return u
		}
	}
	for n, u := range r.Config.Nets {
		if n.Contains(c.IP) {
			return u
		}
	}
	return r.Config.Unmatched
}
