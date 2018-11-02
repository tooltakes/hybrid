package hybridproxy

import (
	"github.com/empirefox/hybrid/pkg/core"
)

type H2Proxy struct {
	client *H2Client
	idx    string
}

func (p *H2Proxy) HttpErr(c *hybridcore.Context, code int, info string) {
	he := &hybridcore.HttpErr{
		Code:       code,
		ClientType: "H2",
		ClientName: p.idx,
		TargetHost: c.HostPort,
		Info:       info,
	}
	c.HttpErr(he)
}

func (p *H2Proxy) Do(c *hybridcore.Context) error { return p.client.Proxy(c, p.idx) }

var _ hybridcore.Proxy = new(H2Proxy)
