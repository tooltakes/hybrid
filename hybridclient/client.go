package hybridclient

import (
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridipfs"
	"go.uber.org/zap"

	inet "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-net"
)

const PathTokenPrefix = "/token/"

type VerifyFunc func(peerID, token []byte) bool

type Client struct {
	log            *zap.Logger
	config         Config
	ipfs           *hybridipfs.Ipfs
	hybrid         *hybrid.Hybrid
	listener       net.Listener
	listeners      []*hybridipfs.Listener
	proxies        map[string]hybrid.Proxy
	fileClients    map[string]*hybrid.FileClient
	fsDisabled     map[string]bool
	routerDisabled map[string]bool
	fileRootDir    string
	ruleRootDir    string
	token          []byte
}

func NewClient(config *Config, hi *hybridipfs.Ipfs, verify VerifyFunc, localServers map[string]http.Handler, log *zap.Logger) (*Client, error) {
	t, err := config.ConfigTree()
	if err != nil {
		return nil, err
	}

	c := Client{
		log:    log,
		config: *config,
		ipfs:   hi,
		proxies: map[string]hybrid.Proxy{
			"DIRECT": hybrid.DirectProxy,
		},
		fileClients:    make(map[string]*hybrid.FileClient, len(config.FileServers)),
		fsDisabled:     parseEnvList("HYBRID_FILE_SERVERS_DISABLED"),
		routerDisabled: parseEnvList("HYBRID_ROUTER_DISABLED"),
		fileRootDir:    t.FilesRootPath,
		ruleRootDir:    t.RulesRootPath,
		token:          []byte(config.Token),
	}

	h2 := hybrid.NewH2Client(hybrid.H2ClientConfig{
		Log: log,
	})

	// IpfsServers
	ipfsToken := []byte(config.Ipfs.Token)
	if len(ipfsToken) == 0 {
		ipfsToken = c.token
	}
	for _, r := range config.IpfsServers {
		s, err := newIpfsServer(r, ipfsToken)
		if err != nil {
			log.Error("newIpfsServer", zap.Error(err))
			return nil, err
		}
		// /hybrid/1.0 /token/ xxx
		protocol := HybridIpfsProtocol + PathTokenPrefix + string(s.Token)
		dialer := &hybrid.H2Dialer{
			Name: s.Name,
			Dial: func() (net.Conn, error) { return c.ipfs.Dial(s.Peer, protocol) },
		}
		h2Proxy, err := h2.AddDialer(dialer)
		if err != nil {
			return nil, err
		}

		c.proxies[s.Name] = h2Proxy
	}

	// FileServers
	for _, s := range config.FileServers {
		name := s.Name
		if name == "" {
			name = s.RootZipName
		}
		fs, err := hybrid.NewFileClient(hybrid.FileClientConfig{
			Log:      log,
			Dev:      s.Dev,
			Disabled: c.fsDisabled[name],
			RootZip:  filepath.Join(c.fileRootDir, s.RootZipName),
			Redirect: s.Redirect,
		})
		if err != nil {
			return nil, err
		}

		c.fileClients[name] = fs
	}

	// HttpProxyServers
	for _, s := range config.HttpProxyServers {
		name := s.Name
		if name == "" {
			name = strings.Replace(s.Host, ":", "-", -1)
		}
		c.proxies[name] = hybrid.NewExistProxy(name, s.Host, s.KeepAlive)
	}

	routers := make([]hybrid.Router, len(config.Routers))
	for i, ri := range config.Routers {
		router, err := c.newRouter(ri)
		if err != nil {
			c.Close()
			return nil, err
		}
		routers[i] = router
	}

	if localServers == nil {
		localServers = make(map[string]http.Handler)
	}
	if config.Ipfs.ApiServerName != "" {
		localServers[config.Ipfs.ApiServerName] = hi.ApiServer()
	}
	if config.Ipfs.GatewayServerName != "" {
		// web: localStorage.setItem('ipfsApi', '/dns4/api-ipfs.hybrid/tcp/80')
		localServers[config.Ipfs.GatewayServerName] = hi.GatewayServer()
	}

	cc := &hybrid.ContextConfig{
		BufferPool:     hybrid.DefaultBufferPool,
		TimeoutForCopy: time.Duration(config.TimeoutForCopyMS) * time.Millisecond,
	}

	c.hybrid = &hybrid.Hybrid{
		Log:           log,
		ContextConfig: cc,
		Routers:       routers,
		Proxies:       c.proxies,
		LocalServers:  localServers,
	}

	listeners := make([]*hybridipfs.Listener, 0, len(config.Ipfs.ListenProtocols))
	for _, p := range config.Ipfs.ListenProtocols {
		// /hybrid/1.0/token/xxx
		tokenPrefix := HybridIpfsProtocol + PathTokenPrefix
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
	c.listeners = listeners

	if c.config.Bind != "" {
		ln, err := net.Listen("tcp", c.config.Bind)
		if err != nil {
			c.log.Error("Listen", zap.Error(err))
			c.Close()
			return nil, err
		}
		c.listener = ln
	}
	return &c, nil
}

func (c *Client) StartServe() <-chan error {
	length := len(c.listeners)
	if c.listener != nil {
		length++
	}

	if length == 0 {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(length)

	out := make(chan error, length)
	for _, ln := range c.listeners {
		go func() {
			out <- c.hybrid.Serve(ln)
			wg.Done()
		}()
	}

	if c.listener != nil {
		go func() {
			err := hybrid.SimpleListenAndServe(c.listener, c.Proxy)
			if err != nil {
				c.log.Error("SimpleListenAndServe", zap.Error(err))
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

func (c *Client) Close() (err error) {
	if c.listener != nil {
		err = c.listener.Close()
	}

	for _, fc := range c.fileClients {
		if err != nil {
			err = fc.Close()
		} else {
			fc.Close()
		}
	}
	return
}

func (c *Client) Proxy(conn net.Conn) {
	defer conn.Close()
	c.hybrid.Proxy(hybrid.NewContextWithConn(c.hybrid.ContextConfig, conn))
}
