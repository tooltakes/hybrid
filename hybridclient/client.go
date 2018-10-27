package hybridclient

import (
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridipfs"
	"github.com/empirefox/hybrid/hybridutils"
	"go.uber.org/zap"
)

type Client struct {
	log            *zap.Logger
	config         Config
	ipfs           *hybridipfs.Ipfs
	hybrid         *hybrid.Hybrid
	listener       net.Listener
	proxies        map[string]hybrid.Proxy
	fileClients    map[string]*hybrid.FileClient
	fsDisabled     map[string]bool
	routerDisabled map[string]bool
	fileRootDir    string
	ruleRootDir    string
	token          []byte
}

func NewClient(config *Config, hi *hybridipfs.Ipfs, localServers map[string]http.Handler, localHandler http.Handler, log *zap.Logger) (*Client, error) {
	t, err := config.ConfigTree()
	if err != nil {
		return nil, err
	}

	c := Client{
		log:    log,
		config: *config,
		ipfs:   hi,
		proxies: map[string]hybrid.Proxy{
			"DIRECT": hybrid.DirectProxy{},
		},
		fileClients:    make(map[string]*hybrid.FileClient, len(config.FileServers)),
		fsDisabled:     parseEnvList("HYBRID_FILE_SERVERS_DISABLED"),
		routerDisabled: parseEnvList("HYBRID_ROUTER_DISABLED"),
		fileRootDir:    t.FilesRootPath,
		ruleRootDir:    t.RulesRootPath,
		token:          []byte(config.Token),
	}

	// tls1.3 scalar
	scalar, err := hybridutils.DecodeKey32(config.ScalarHex)
	if err != nil {
		return nil, err
	}

	h2 := hybrid.NewH2Client(hybrid.H2ClientConfig{
		Log:    log,
		Scalar: scalar,
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
			Dial:         func() (net.Conn, error) { return c.ipfs.Dial(s.Peer, protocol) },
			NoTLS:        true,
			ClientScalar: nil,
			ServerPubkey: nil,
			NoAuth:       true,
			Token:        s.Token,
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
		c.proxies[name] = hybrid.NewExistProxy(s.Host, s.KeepAlive)
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

	c.hybrid = &hybrid.Hybrid{
		Routers:      routers,
		Proxies:      c.proxies,
		LocalServers: localServers,
		LocalHandler: localHandler,
	}

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
	if c.listener != nil {
		errc := make(chan error)
		go func() {
			err := hybrid.SimpleListenAndServe(c.listener, c.Proxy)
			if err != nil {
				c.log.Error("SimpleListenAndServe", zap.Error(err))
			}
			errc <- err
		}()
		return errc
	}
	return nil
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

func (c *Client) Proxy(conn net.Conn) { c.hybrid.Proxy(conn) }
