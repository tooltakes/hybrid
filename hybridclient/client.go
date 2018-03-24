package hybridclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/toxenums"
	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridtox"
	"go.uber.org/zap"
)

type Client struct {
	log            *zap.Logger
	config         Config
	tox            *tox.Tox
	toxNodes       []tox.BootstrapNode
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
	c := Client{
		log:      log,
		config:   config,
		toxNodes: hybridtox.ToxNodes,
		proxies: map[string]hybrid.Proxy{
			"DIRECT": hybrid.DirectProxy{},
		},
		fileClients:    make(map[string]*hybrid.FileClient, len(config.FileServers)),
		fsDisabled:     parseEnvList("HYBRID_FILE_SERVERS_DISABLED"),
		routerDisabled: parseEnvList("HYBRID_ROUTER_DISABLED"),
		fileRootDir:    os.ExpandEnv(config.FileRootDir),
		ruleRootDir:    os.ExpandEnv(config.RuleRootDir),
		token:          []byte(config.Token),
	}

	if len(config.ToxNodes) != 0 {
		toxNodes := make([]tox.BootstrapNode, len(config.ToxNodes))
		for i, n := range config.ToxNodes {
			pubkey, err := tox.DecodePubkey(n.Pubkey)
			if err != nil {
				return nil, err
			}
			toxNodes[i] = tox.BootstrapNode{
				Addr:    n.Addr,
				Port:    n.Port,
				TcpPort: n.TcpPort,
				Pubkey:  *pubkey,
			}
		}
		c.toxNodes = toxNodes
	}

	// tox and tls1.3 use the same scalar
	scalar, err := tox.DecodeSecret(config.ScalarHex)
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

	// ToxServers parse
	toxServersSize := len(config.ToxServers)
	toxServers := make([]*[tox.ADDRESS_SIZE]byte, toxServersSize)
	toxTokens := make(map[[tox.PUBLIC_KEY_SIZE]byte][]byte, toxServersSize)
	toxNames := make(map[string]*[tox.ADDRESS_SIZE]byte, toxServersSize)
	for i, s := range config.ToxServers {
		ts, err := newToxServer(s, c.token)
		if err != nil {
			log.Error("newToxServer", zap.Error(err))
			return nil, err
		}

		toxServers[i] = ts.Address
		pubkey := tox.ToPubkey(ts.Address)
		toxTokens[*pubkey] = ts.Token
		toxNames[ts.Name] = ts.Address
	}

	// FileServers
	for _, s := range config.FileServers {
		name := s.Name
		if name == "" {
			name = s.DirName
		}
		fs, err := hybrid.NewFileClient(hybrid.FileClientConfig{
			Log:      log,
			Disabled: c.fsDisabled[name],
			DirPath:  filepath.Join(c.fileRootDir, s.DirName),
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
		c.proxies[name] = hybrid.NewExistProxy(s.Host)
	}

	// tox
	if len(config.ToxServers) > 0 {
		t, err := tox.NewTox(&tox.ToxOptions{
			Savedata_type:           toxenums.TOX_SAVEDATA_TYPE_SECRET_KEY,
			Savedata_data:           scalar[:],
			Tcp_port:                33445,
			NospamIfSecretType:      config.ToxNospam,
			ProxyToNoneIfErr:        true,
			AutoTcpPortIfErr:        true,
			DisableTcpPortIfAutoErr: true,
			PingUnit:                time.Second,
		})
		if err != nil {
			log.Info("NewTox", zap.Error(err))
			t.Kill()
			return nil, err
		}
		c.tox = t

		tt := hybridtox.NewToxTCP(hybridtox.ToxTCPConfig{
			Log:             log,
			Tox:             t,
			Supers:          nil,
			Servers:         toxServers,
			RequestToken:    func(pubkey *[tox.PUBLIC_KEY_SIZE]byte) []byte { return toxTokens[*pubkey] },
			ValidateRequest: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte, message []byte) bool { return false },

			// every friend may fire multi times
			OnFriendAddErr: func(address *[tox.ADDRESS_SIZE]byte, err error) {
				log.Info("OnFriendAddErr", zap.Error(err))
			},

			// controll tox only
			OnSupperAction: func(friendNumber uint32, action []byte) {
				log.Info("OnSupperAction", zap.ByteString("action", action))
			},
		})

		// ToxServers proxy
		for name, address := range toxNames {
			dialer := &hybrid.H2Dialer{
				Dial: func() (c net.Conn, err error) {
					ctx, cancel := context.WithTimeout(context.Background(), hybridtox.DefaultDialTimeout)
					defer cancel()
					return tt.DialContext(ctx, address)
				},
				NoTLS:        true,
				ClientScalar: nil,
				ServerPubkey: nil,
				NoAuth:       true,
				Token:        nil,
			}
			h2Proxy, err := h2.AddDialer(dialer)
			if err != nil {
				t.Kill()
				return nil, err
			}

			c.proxies[name] = h2Proxy
		}
	}

	routers := make([]hybrid.Router, len(config.Routers))
	for i, ri := range config.Routers {
		router, err := c.newRouter(ri)
		if err != nil {
			if c.tox != nil {
				c.tox.Kill()
			}
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

func (c *Client) Run() error {
	if c.tox != nil {
		result := c.tox.BootstrapNodes_l(c.toxNodes)
		if result.Error() != nil {
			c.log.Error("BootstrapNodes_l", zap.Error(result.Error()))
			return result.Error()
		}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", c.config.Expose))
	if err != nil {
		c.log.Error("Listen", zap.Error(err))
		return err
	}
	defer ln.Close()
	c.listener = ln

	if c.tox != nil {
		go c.tox.Run()
	}

	err = hybrid.SimpleListenAndServe(ln, c.Proxy)
	if err != nil {
		c.log.Error("SimpleListenAndServe", zap.Error(err))
		if c.tox != nil {
			c.tox.StopAndKill()
		}
	}
	return err
}

func (c *Client) StopAndKill() {
	if c.listener != nil {
		c.listener.Close()
	} else if c.tox != nil {
		c.tox.Kill()
	}
}

func (c *Client) Proxy(conn net.Conn) { c.hybrid.Proxy(conn) }

func (c *Client) Tox() *tox.Tox { return c.tox }
