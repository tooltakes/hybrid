package hybridcore

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
	keepAlive bool
}

func NewExistProxy(name, host string, keepAlive bool) *ExistProxy {
	return &ExistProxy{name, host, keepAlive}
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
func (p *ExistProxy) Do(c *Context) error { return c.ProxyUp(p.host, p.keepAlive) }

var DirectProxy Proxy = directProxy{}
var _ Proxy = new(ExistProxy)
