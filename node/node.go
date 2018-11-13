package node

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/bufpool"
	"github.com/empirefox/hybrid/pkg/core"
	"github.com/empirefox/hybrid/pkg/ipfs"
	"github.com/empirefox/hybrid/pkg/netutil"
	"github.com/empirefox/hybrid/pkg/proxy"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	inet "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-net"
)

const PathTokenPrefix = "/token/"

var (
	ErrConfigBindNotSet = errors.New("Config.Bind not set")
)

type VerifyFunc func(peerID, token []byte) bool

type Config struct {
	Log    *zap.Logger
	Config *config.Config
	Ipfs   *ipfs.Ipfs
	Verify VerifyFunc

	// LocalServers can be nil
	LocalServers map[string]http.Handler

	// ConfigBindId as key of StartProxy for Config.Bind.
	// The value should not be used by user.
	ConfigBindId uint32
}

type Node struct {
	log            *zap.Logger
	c              config.Config
	ipfs           *ipfs.Ipfs
	core           *core.Core
	ipfsListeners  []*ipfs.Listener
	groupListeners sync.Map
	configBindId   uint32
	proxies        map[string]core.Proxy
	fileClients    map[string]*proxy.FileProxyRouterClient
	fsDisabled     map[string]bool
	routerDisabled map[string]bool
	fileRootDir    string
	ruleRootDir    string
	token          []byte
	eg             errgroup.Group
	closeOnce      sync.Once
}

func New(nc Config) (*Node, error) {
	c := nc.Config
	localServers := nc.LocalServers
	log := nc.Log

	t, err := nc.Config.ConfigTree()
	if err != nil {
		return nil, err
	}

	n := Node{
		log:          log,
		c:            *c,
		ipfs:         nc.Ipfs,
		configBindId: nc.ConfigBindId,
		proxies: map[string]core.Proxy{
			"DIRECT": core.DirectProxy,
		},
		fileClients:    make(map[string]*proxy.FileProxyRouterClient, len(c.FileServers)),
		fsDisabled:     parseEnvList("HYBRID_FILE_SERVERS_DISABLED"),
		routerDisabled: parseEnvList("HYBRID_ROUTER_DISABLED"),
		fileRootDir:    t.FilesRootPath,
		ruleRootDir:    t.RulesRootPath,
		token:          []byte(nc.Config.Token),
	}

	h2 := proxy.NewH2Client(proxy.H2ClientConfig{
		Log: log,
	})

	// IpfsServers
	ipfsToken := []byte(c.Ipfs.Token)
	if len(ipfsToken) == 0 {
		ipfsToken = n.token
	}
	for _, r := range c.IpfsServers {
		s, err := newIpfsServer(r, ipfsToken)
		if err != nil {
			log.Error("newIpfsServer", zap.Error(err))
			return nil, err
		}
		// /hybrid/1.0 /token/ xxx
		protocol := config.HybridIpfsProtocol + PathTokenPrefix + string(s.Token)
		dialer := &proxy.H2Dialer{
			Name: s.Name,
			Dial: func() (net.Conn, error) { return n.ipfs.Dial(s.Peer, protocol) },
		}
		h2Proxy, err := h2.AddDialer(dialer)
		if err != nil {
			return nil, err
		}

		n.proxies[s.Name] = h2Proxy
	}

	// FileServers
	// newNetRouter will search with FileTest
	for _, s := range c.FileServers {
		name := s.Name
		if name == "" {
			name = s.RootZipName
		}
		fs, err := proxy.NewFileProxyRouterClient(proxy.FileClientConfig{
			Log:      log,
			Dev:      s.Dev,
			Disabled: n.fsDisabled[name],
			RootZip:  filepath.Join(n.fileRootDir, s.RootZipName),
			Redirect: s.Redirect,
		})
		if err != nil {
			return nil, err
		}

		n.fileClients[name] = fs
	}

	// HttpProxyServers
	for _, s := range c.HttpProxyServers {
		name := s.Name
		if name == "" {
			name = strings.Replace(s.Host, ":", "-", -1)
		}
		p, err := core.NewExistProxy(name, s.Host, s.KeepAlive)
		if err != nil {
			return nil, err
		}
		n.proxies[name] = p
	}

	routers := make([]core.Router, len(c.Routers))
	for i, ri := range c.Routers {
		router, err := n.newRouter(ri)
		if err != nil {
			n.Close()
			return nil, err
		}
		routers[i] = router
	}

	if localServers == nil {
		localServers = make(map[string]http.Handler)
	}
	if c.Ipfs.ApiServerName != "" {
		// web: localStorage.setItem('ipfsApi', '/dns4/api.ipfs.with.hybrid/tcp/80')
		localServers[c.Ipfs.ApiServerName] = n.ipfs.ApiServer()
	}
	if c.Ipfs.GatewayServerName != "" {
		localServers[c.Ipfs.GatewayServerName] = n.ipfs.GatewayServer()
	}

	cc := &core.ContextConfig{
		Transport:     http.DefaultTransport,
		BufferPool:    bufpool.Default,
		FlushInterval: time.Duration(c.FlushIntervalMS) * time.Millisecond,
	}

	n.core = &core.Core{
		Log:           log,
		ContextConfig: cc,
		Routers:       routers,
		Proxies:       n.proxies,
		LocalServers:  localServers,
	}

	ipfsListeners := make([]*ipfs.Listener, 0, len(c.Ipfs.ListenProtocols))
	for _, p := range c.Ipfs.ListenProtocols {
		// /hybrid/1.0/token/xxx
		tokenPrefix := config.HybridIpfsProtocol + PathTokenPrefix
		match := func(protocol string) bool { return strings.HasPrefix(protocol, tokenPrefix) }
		// TODO what if p!=HybridIpfsProtocol
		ln, err := n.ipfs.Listen(p, match)
		if err != nil {
			return nil, err
		}

		ln.SetVerify(func(is inet.Stream) bool {
			target := []byte(is.Conn().RemotePeer().Pretty())
			token := []byte(strings.TrimPrefix(string(is.Protocol()), tokenPrefix))
			return nc.Verify(target, token)
		})
		ipfsListeners = append(ipfsListeners, ln)
	}
	n.ipfsListeners = ipfsListeners

	for _, ln := range n.ipfsListeners {
		n.eg.Go(func() error { return n.core.Serve(ln) })
	}

	return &n, nil
}

func (n *Node) StartConfigProxy() error {
	if n.c.Bind == "" {
		return ErrConfigBindNotSet
	}

	ln, err := net.Listen("tcp", n.c.Bind)
	if err != nil {
		return err
	}

	n.StartProxy(n.configBindId, ln)
	return nil
}

func (n *Node) StopConfigProxy() error {
	return n.StopListener(n.configBindId)
}

func (n *Node) StartProxy(uniqueId uint32, ln net.Listener) {
	n.groupListeners.Store(uniqueId, ln)
	n.eg.Go(func() error {
		defer n.groupListeners.Delete(uniqueId)
		return netutil.SimpleServe(ln, n.proxy)
	})
}

func (n *Node) StartIpfsApi(uniqueId uint32, ln net.Listener) {
	n.groupListeners.Store(uniqueId, ln)
	n.eg.Go(func() error {
		defer n.groupListeners.Delete(uniqueId)
		return http.Serve(ln, n.ipfs.ApiServer())
	})
}

func (n *Node) StartIpfsGateway(uniqueId uint32, ln net.Listener) {
	n.groupListeners.Store(uniqueId, ln)
	n.eg.Go(func() error {
		defer n.groupListeners.Delete(uniqueId)
		return http.Serve(ln, n.ipfs.GatewayServer())
	})
}

func (n *Node) StopListener(uniqueId uint32) error {
	value, ok := n.groupListeners.Load(uniqueId)
	if !ok {
		return nil
	}
	return value.(net.Listener).Close()
}

func (n *Node) ErrGroupWait() error { return n.eg.Wait() }
func (n *Node) Go(f func() error)   { n.eg.Go(f) }

func (n *Node) Close() (err error) {
	n.closeOnce.Do(func() {
		for _, ln := range n.ipfsListeners {
			ln.Close()
		}
		n.groupListeners.Range(func(key, value interface{}) bool {
			value.(net.Listener).Close()
			return true
		})
		for _, fc := range n.fileClients {
			if err != nil {
				err = fc.Close()
			} else {
				fc.Close()
			}
		}
	})
	return
}

func (n *Node) proxy(conn net.Conn) {
	defer conn.Close()
	ctx, err := core.NewContextWithConn(n.core.ContextConfig, conn)
	if err != nil {
		he := core.HttpErr{
			Code:       http.StatusBadRequest,
			ClientType: "Hybrid",
			ClientName: "CTX",
			TargetHost: "",
			Info:       err.Error(),
		}
		he.Write(conn)
		return
	}
	n.core.Proxy(ctx)
}
