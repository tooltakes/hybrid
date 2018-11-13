package core

import (
	"fmt"
	"net/http"
	"net/url"
)

type Router interface {
	// Disabled will be ignored
	Disabled() bool

	// Route rules: nil: not handle, empty direct
	Route(c *Context) Proxy
}

type Proxy interface {
	Do(c *Context) error
	HttpErr(c *Context, code int, info string)
}

type directProxy struct{}

func (directProxy) HttpErr(c *Context, code int, info string) {
	he := &HttpErr{
		Code:       code,
		ClientType: "Direct",
		ClientName: "Host:",
		TargetHost: c.HostPort,
		Info:       info,
	}
	c.HttpErr(he)
}

type ExistProxy struct {
	name      string
	host      string
	transport http.RoundTripper
	keepAlive bool
}

// TODO add https socks5 support
func NewExistProxy(name, host string, keepAlive bool) (*ExistProxy, error) {
	proxyURL, err := url.Parse("http://" + host)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", host, err)
	}

	tp := *(http.DefaultTransport.(*http.Transport))
	tp.Proxy = http.ProxyURL(proxyURL)
	return &ExistProxy{name, host, &tp, keepAlive}, nil
}

func (p *ExistProxy) HttpErr(c *Context, code int, info string) {
	he := &HttpErr{
		Code:       code,
		ClientType: "Exist",
		ClientName: p.name,
		TargetHost: c.HostPort,
		Info:       info,
	}
	c.HttpErr(he)
}

func (p directProxy) Do(c *Context) error { return c.Direct() }
func (p *ExistProxy) Do(c *Context) error { return c.ProxyUp(p.host, p.transport, p.keepAlive) }

var DirectProxy Proxy = directProxy{}
var _ Proxy = new(ExistProxy)
