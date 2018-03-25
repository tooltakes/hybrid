package hybrid

import (
	"net"
)

type IPNetRouter struct {
	Skip bool

	IPs  []net.IP
	Nets []*net.IPNet

	// FileClient test Matched host file, then proxy it if test ok.
	FileClient *FileClient

	Matched   Proxy
	Unmatched Proxy
}

func (r *IPNetRouter) Disabled() bool { return r.Skip }

func (r *IPNetRouter) Route(c *Context) Proxy {
	for _, i := range r.IPs {
		if i.Equal(c.IP) {
			if p := r.FileClient.Route(c); p != nil {
				return p
			}
			return r.Matched
		}
	}
	for _, n := range r.Nets {
		if n.Contains(c.IP) {
			if p := r.FileClient.Route(c); p != nil {
				return p
			}
			return r.Matched
		}
	}
	return r.Unmatched
}
