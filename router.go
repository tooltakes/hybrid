package hybrid

import (
	"net/url"
)

var (
	NEXT, _   = url.Parse("//NEXT")
	DIRECT, _ = url.Parse("//DIRECT")
	EXIST, _  = url.Parse("//EXIST")
	RETURN, _ = url.Parse("//RETURN")
)

type Router interface {

	// will be ignored
	Disabled() bool

	// will call ProxyUp
	Exist() string

	// Route rules: nil: not handle, empty direct
	Route(c *Context) (proxy *url.URL)
}

type RouteClient struct {
	Router
	*Client
}

func NewRouteClient(r Router, c *Client) RouteClient {
	return RouteClient{
		Router: r,
		Client: c,
	}
}
