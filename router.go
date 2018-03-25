package hybrid

type Router interface {
	// Disabled will be ignored
	Disabled() bool

	// Route rules: nil: not handle, empty direct
	Route(c *Context) Proxy
}

type Proxy interface {
	Do(c *Context)
}

type DirectProxy struct{}

type ExistProxy struct {
	host      string
	keepAlive bool
}

func NewExistProxy(host string, keepAlive bool) *ExistProxy {
	return &ExistProxy{host, keepAlive}
}

type H2Proxy struct {
	client *H2Client
	idx    string
}

func (DirectProxy) Do(c *Context)   { c.Direct() }
func (p *ExistProxy) Do(c *Context) { c.ProxyUp(p.host, p.keepAlive) }
func (p *H2Proxy) Do(c *Context)    { p.client.Proxy(c, p.idx) }

var _ Proxy = DirectProxy{}
var _ Proxy = NewExistProxy("", false)
var _ Proxy = new(H2Proxy)
