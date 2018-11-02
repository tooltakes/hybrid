package hybridnode

import (
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/bufpool"
	"github.com/empirefox/hybrid/pkg/core"
	"github.com/empirefox/hybrid/pkg/http"
	"github.com/empirefox/hybrid/pkg/ipfs"
	"github.com/empirefox/hybrid/pkg/proxy"
	"go.uber.org/zap"

	inet "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-net"
)

const PathTokenPrefix = "/token/"

type VerifyFunc func(peerID, token []byte) bool

type Node struct {
	log            *zap.Logger
	config         hybridconfig.Config
	ipfs           *hybridipfs.Ipfs
	core           *hybridcore.Core
	listener       net.Listener
	listeners      []*hybridipfs.Listener
	proxies        map[string]hybridcore.Proxy
	fileClients    map[string]*hybridproxy.FileProxyRouterClient
	fsDisabled     map[string]bool
	routerDisabled map[string]bool
	fileRootDir    string
	ruleRootDir    string
	token          []byte
}

func New(config *hybridconfig.Config, hi *hybridipfs.Ipfs, verify VerifyFunc, localServers map[string]http.Handler, log *zap.Logger) (*Node, error) {
	t, err := config.ConfigTree()
	if err != nil {
		return nil, err
	}

	n := Node{
		log:    log,
		config: *config,
		ipfs:   hi,
		proxies: map[string]hybridcore.Proxy{
			"DIRECT": hybridcore.DirectProxy,
		},
		fileClients:    make(map[string]*hybridproxy.FileProxyRouterClient, len(config.FileServers)),
		fsDisabled:     parseEnvList("HYBRID_FILE_SERVERS_DISABLED"),
		routerDisabled: parseEnvList("HYBRID_ROUTER_DISABLED"),
		fileRootDir:    t.FilesRootPath,
		ruleRootDir:    t.RulesRootPath,
		token:          []byte(config.Token),
	}

	h2 := hybridproxy.NewH2Client(hybridproxy.H2ClientConfig{
		Log: log,
	})

	// IpfsServers
	ipfsToken := []byte(config.Ipfs.Token)
	if len(ipfsToken) == 0 {
		ipfsToken = n.token
	}
	for _, r := range config.IpfsServers {
		s, err := newIpfsServer(r, ipfsToken)
		if err != nil {
			log.Error("newIpfsServer", zap.Error(err))
			return nil, err
		}
		// /hybrid/1.0 /token/ xxx
		protocol := hybridconfig.HybridIpfsProtocol + PathTokenPrefix + string(s.Token)
		dialer := &hybridproxy.H2Dialer{
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
	for _, s := range config.FileServers {
		name := s.Name
		if name == "" {
			name = s.RootZipName
		}
		fs, err := hybridproxy.NewFileProxyRouterClient(hybridproxy.FileClientConfig{
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
	for _, s := range config.HttpProxyServers {
		name := s.Name
		if name == "" {
			name = strings.Replace(s.Host, ":", "-", -1)
		}
		n.proxies[name] = hybridcore.NewExistProxy(name, s.Host, s.KeepAlive)
	}

	routers := make([]hybridcore.Router, len(config.Routers))
	for i, ri := range config.Routers {
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
	if config.Ipfs.ApiServerName != "" {
		// web: localStorage.setItem('ipfsApi', '/dns4/api.ipfs.with.hybrid/tcp/80')
		localServers[config.Ipfs.ApiServerName] = hi.ApiServer()
	}
	if config.Ipfs.GatewayServerName != "" {
		localServers[config.Ipfs.GatewayServerName] = hi.GatewayServer()
	}

	cc := &hybridcore.ContextConfig{
		BufferPool:     hybridbufpool.Default,
		TimeoutForCopy: time.Duration(config.TimeoutForCopyMS) * time.Millisecond,
	}

	n.core = &hybridcore.Core{
		Log:           log,
		ContextConfig: cc,
		Routers:       routers,
		Proxies:       n.proxies,
		LocalServers:  localServers,
	}

	listeners := make([]*hybridipfs.Listener, 0, len(config.Ipfs.ListenProtocols))
	for _, p := range config.Ipfs.ListenProtocols {
		// /hybrid/1.0/token/xxx
		tokenPrefix := hybridconfig.HybridIpfsProtocol + PathTokenPrefix
		match := func(protocol string) bool { return strings.HasPrefix(protocol, tokenPrefix) }
		// TODO what if p!=HybridIpfsProtocol
		ln, err := hi.Listen(p, match)
		if err != nil {
			return nil, err
		}

		ln.SetVerify(func(is inet.Stream) bool {
			target := []byte(is.Conn().RemotePeer().Pretty())
			token := []byte(strings.TrimPrefix(string(is.Protocol()), tokenPrefix))
			return verify(target, token)
		})
		listeners = append(listeners, ln)
	}
	n.listeners = listeners

	if n.config.Bind != "" {
		ln, err := net.Listen("tcp", n.config.Bind)
		if err != nil {
			n.log.Error("Listen", zap.Error(err))
			n.Close()
			return nil, err
		}
		n.listener = ln
	}
	return &n, nil
}

func (n *Node) StartServe() <-chan error {
	length := len(n.listeners)
	if n.listener != nil {
		length++
	}

	if length == 0 {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(length)

	out := make(chan error, length)
	for _, ln := range n.listeners {
		go func() {
			out <- n.core.Serve(ln)
			wg.Done()
		}()
	}

	if n.listener != nil {
		go func() {
			err := hybridhttp.SimpleListenAndServe(n.listener, n.Proxy)
			if err != nil {
				n.log.Error("SimpleListenAndServe", zap.Error(err))
			}
			out <- err
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (n *Node) Close() (err error) {
	if n.listener != nil {
		err = n.listener.Close()
	}

	for _, fc := range n.fileClients {
		if err != nil {
			err = fc.Close()
		} else {
			fc.Close()
		}
	}
	return
}

func (n *Node) Proxy(conn net.Conn) {
	defer conn.Close()
	c, err := hybridcore.NewContextWithConn(n.core.ContextConfig, conn)
	if err != nil {
		he := hybridcore.HttpErr{
			Code:       http.StatusBadRequest,
			ClientType: "Hybrid",
			ClientName: "CTX",
			TargetHost: "",
			Info:       err.Error(),
		}
		he.Write(conn)
		return
	}
	n.core.Proxy(c)
}
