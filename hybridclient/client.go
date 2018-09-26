package hybridclient

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridutils"
	"go.uber.org/zap"
)

type Client struct {
	log            *zap.Logger
	config         Config
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

func NewClient(config Config, localHandler http.Handler, log *zap.Logger) (*Client, error) {
	t := NewConfigTree(config.BaseDir)

	c := Client{
		log:    log,
		config: config,
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

	// TcpServers
	for _, s := range config.TcpServers {
		ts, err := newTcpServer(s, c.token)
		if err != nil {
			log.Error("newTcpServer", zap.Error(err))
			return nil, err
		}
		dialer := &hybrid.H2Dialer{
			Dial:         func() (c net.Conn, err error) { return net.Dial("tcp", ts.Addr) },
			NoTLS:        false,
			ClientScalar: ts.ClientScalar,
			ServerPubkey: ts.ServerPubkey,
			NoAuth:       false,
			Token:        ts.Token,
		}
		h2Proxy, err := h2.AddDialer(dialer)
		if err != nil {
			return nil, err
		}

		c.proxies[ts.Name] = h2Proxy
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
			return nil, err
		}
		routers[i] = router
	}

	c.hybrid = &hybrid.Hybrid{
		Routers:      routers,
		Proxies:      c.proxies,
		LocalHandler: localHandler,
	}
	return &c, nil
}

func (c *Client) InitListener() error {
	if c.listener == nil {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", c.config.Expose))
		if err != nil {
			c.log.Error("Listen", zap.Error(err))
			return err
		}
		c.listener = ln
	}
	return nil
}

func (c *Client) Run() error {
	err := hybrid.SimpleListenAndServe(c.listener, c.Proxy)
	if err != nil {
		c.log.Error("SimpleListenAndServe", zap.Error(err))
	}
	return err
}

func (c *Client) StopAndKill() {
	if c.listener != nil {
		c.listener.Close()
	}

	for _, fc := range c.fileClients {
		fc.Close()
	}
}

func (c *Client) Proxy(conn net.Conn) { c.hybrid.Proxy(conn) }
