package hybridipfs

import (
	"net"
	"net/http"
	"sync/atomic"

	oldcmds "github.com/ipsn/go-ipfs/commands"
	"github.com/ipsn/go-ipfs/core"
	"github.com/ipsn/go-ipfs/core/corehttp"

	ma "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr"
	manet "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr-net"
)

func (hi *Ipfs) ApiServer() http.Handler     { return &hi.apiServer }
func (hi *Ipfs) GatewayServer() http.Handler { return &hi.gatewayServer }

type HttpServer struct {
	handler atomic.Value
}

// ServeHTTP implement handler
func (hs *HttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	hs.handler.Load().(http.HandlerFunc)(w, req)
}

func (hs *HttpServer) SetOnline(handler http.HandlerFunc, err error) {
	if err != nil {
		hs.handler.Store(newErrGatewayHanlder(err))
	} else {
		hs.handler.Store(handler)
	}
}

func (hs *HttpServer) SetOffline(err error) {
	if err != nil {
		hs.handler.Store(newErrGatewayHanlder(err))
	} else {
		hs.handler.Store(offlineGatewayHandler)
	}
}

var offlineGatewayHandler http.HandlerFunc = func(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
}

func newErrGatewayHanlder(err error) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
	}
}

func newApiHandler(node *core.IpfsNode, cctx *oldcmds.Context, c *Config) (http.HandlerFunc, error) {
	var opts = []corehttp.ServeOption{
		corehttp.CommandsOption(*cctx), // /api/v0
		corehttp.WebUIOption,           // /webui
		corehttp.GatewayOption(true, "/ipfs", "/ipns"),
		corehttp.LogOption(),
	}
	return newHttpHandler(node, c.FakeApiListenAddr, opts)
}

func newGatewayHandler(node *core.IpfsNode, cctx *oldcmds.Context, c *Config) (http.HandlerFunc, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, err
	}

	var opts = []corehttp.ServeOption{
		IPNSHostnameOption(c.ExcludeIPNS),
		corehttp.GatewayOption(cfg.Gateway.Writable, "/ipfs", "/ipns"),
		corehttp.CheckVersionOption(),
		corehttp.CommandsROOption(*cctx), // /api/v0
	}
	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	return newHttpHandler(node, c.GatewayListenAddr, opts)
}

func newHttpHandler(node *core.IpfsNode, maddr ma.Multiaddr, opts []corehttp.ServeOption) (http.HandlerFunc, error) {
	ln, _, err := NewListenDialer(maddr, maddr)
	if err != nil {
		return nil, err
	}
	defer ln.Close()

	handler, err := makeHandler(node, manet.NetListener(ln), opts...)
	if err != nil {
		return nil, err
	}

	return handler.ServeHTTP, nil
}

// makeHandler turns a list of ServeOptions into a http.Handler that implements
// all of the given options, in order.
func makeHandler(n *core.IpfsNode, l net.Listener, options ...corehttp.ServeOption) (http.Handler, error) {
	topMux := http.NewServeMux()
	mux := topMux
	for _, option := range options {
		var err error
		mux, err = option(n, l, mux)
		if err != nil {
			return nil, err
		}
	}
	return topMux, nil
}
