package proxy

import (
	"github.com/empirefox/hybrid/pkg/core"
)

type H2Proxy struct {
	client *H2Client
	idx    string
}

func (p *H2Proxy) HttpErr(c *core.Context, code int, info string) {
	he := &core.HttpErr{
		Code:       code,
		ClientType: "H2",
		ClientName: p.idx,
		TargetHost: c.HostPort,
		Info:       info,
	}
	c.HttpErr(he)
}

func (p *H2Proxy) Do(c *core.Context) error { return p.client.Proxy(c, p.idx) }

var _ core.Proxy = new(H2Proxy)
