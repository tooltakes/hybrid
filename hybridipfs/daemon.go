package hybridipfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/net/context"

	oldcmds "github.com/ipsn/go-ipfs/commands"
	"github.com/ipsn/go-ipfs/core"
	coreiface "github.com/ipsn/go-ipfs/core/coreapi/interface"
	"github.com/ipsn/go-ipfs/core/corerepo"
	"github.com/ipsn/go-ipfs/plugin/loader"
	"github.com/ipsn/go-ipfs/repo/fsrepo"

	config "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-config"
	logging "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-log"
	ma "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr"
)

const (
	routingOptionDHTClientKwd = "dhtclient"
	routingOptionDHTKwd       = "dht"
	routingOptionNoneKwd      = "none"
)

var (
	// ErrIsOffline is returned when an online operation was done offline.
	ErrIsOffline        = errors.New("Ipfs is offline")
	ErrIsOnline         = errors.New("Ipfs is online")
	ErrProtocolListened = errors.New("Protocol already listened")
)

var log = logging.Logger("hybridipfs")

type StateChangedFunc func(stateLocked *core.IpfsNode)

type Config struct {
	FakeApiListenAddr ma.Multiaddr
	GatewayListenAddr ma.Multiaddr
	ExcludeIPNS       func(host string) bool

	RepoPath         string
	Profile          []string // optional
	AutoMigrate      bool
	EnableIPNSPubSub bool
	EnableFloodSub   bool
	EnableMultiplex  bool
}

// Ipfs remembers the settings needed for accessing the ipfs daemon.
type Ipfs struct {
	config Config
	ctx    context.Context

	mu             sync.Mutex
	ipfsNode       *core.IpfsNode
	api            coreiface.CoreAPI
	apiServer      HttpServer
	gatewayServer  HttpServer
	listeners      map[string]*Listener
	onConnected    StateChangedFunc
	onDisconnected StateChangedFunc
	cancel         context.CancelFunc
}

func NewIpfs(ctx context.Context, c *Config) (*Ipfs, error) {
	if c.RepoPath == "" {
		repoPath, err := fsrepo.BestKnownPath()
		if err != nil {
			return nil, err
		}
		c.RepoPath = repoPath
	}

	err := CheckPlugins(c.RepoPath)
	if err != nil {
		return nil, err
	}

	ipfsNode, err := createOfflineNode(ctx, c)
	if err != nil {
		return nil, err
	}

	hi := &Ipfs{
		config: *c,
		ctx:    ctx,

		ipfsNode:  ipfsNode,
		listeners: make(map[string]*Listener),
	}
	hi.apiServer.SetOffline(nil)
	hi.gatewayServer.SetOffline(nil)
	return hi, nil
}

func (hi *Ipfs) IsOnline() bool {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	return hi.isOnline()
}

func (hi *Ipfs) isOnline() bool {
	return hi.ipfsNode.OnlineMode()
}

func (hi *Ipfs) Connect() error {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	if hi.isOnline() {
		return nil
	}

	cctx := newCctx(hi.config.RepoPath)
	ctx, cancel := context.WithCancel(context.Background())
	var onErr = func() {
		cancel()
		cctx.Close()
	}
	err := startDaemon(ctx, cctx, &hi.config)
	if err != nil {
		onErr()
		return err
	}

	ipfsNode, err := cctx.GetNode()
	if err != nil {
		onErr()
		return err
	}

	api, err := cctx.GetApi()
	if err != nil {
		onErr()
		return err
	}

	handler, err := newApiHandler(ipfsNode, cctx, &hi.config)
	hi.apiServer.SetOnline(handler, err)
	if err != nil {
		return err
	}

	handler, err = newGatewayHandler(ipfsNode, cctx, &hi.config)
	hi.gatewayServer.SetOnline(handler, err)
	if err != nil {
		return err
	}

	if hi.ipfsNode != nil {
		hi.ipfsNode.Close()
	}
	hi.ipfsNode = ipfsNode
	hi.api = api
	hi.cancel = cancel

	// server behavior
	for _, ln := range hi.listeners {
		hi.setStreamHandlerToDaemonHost(ln)
	}

	if hi.onConnected != nil {
		hi.onConnected(ipfsNode)
	}
	return nil
}

func (hi *Ipfs) Disconnect() (err error) {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	if !hi.isOnline() {
		return ErrIsOffline
	}

	hi.cancel()
	hi.ipfsNode.Close()
	hi.ipfsNode, err = createOfflineNode(hi.ctx, &hi.config)
	hi.apiServer.SetOffline(err)
	hi.gatewayServer.SetOffline(err)

	if hi.onDisconnected != nil {
		hi.onDisconnected(hi.ipfsNode)
	}
	return
}

func (hi *Ipfs) Connected(onConnected StateChangedFunc) {
	hi.mu.Lock()
	defer hi.mu.Unlock()
	hi.onConnected = onConnected
}

// Disconnected ipfsNode maybe nil
func (hi *Ipfs) Disconnected(onDisconnected StateChangedFunc) {
	hi.mu.Lock()
	defer hi.mu.Unlock()
	hi.onDisconnected = onDisconnected
}

func CheckPlugins(repoPath string) error {
	// check if repo is accessible before loading plugins
	ok, err := checkPermissions(repoPath)
	if err != nil {
		return err
	}
	if ok {
		pluginpath := filepath.Join(repoPath, "plugins")
		if _, err = loader.LoadPlugins(pluginpath); err != nil {
			return err
		}
	}

	return nil
}

func checkPermissions(path string) (bool, error) {
	_, err := os.Open(path)
	if os.IsNotExist(err) {
		// repo does not exist yet - don't load plugins, but also don't fail
		return false, nil
	}
	if os.IsPermission(err) {
		// repo is not accessible. error out.
		return false, err
	}

	return true, nil
}

func createOfflineNode(ctx context.Context, c *Config) (*core.IpfsNode, error) {
	repo, err := InitDefaultOrMigrateRepoIfNeeded(c)
	if err != nil {
		return nil, err
	}

	cfg := &core.BuildCfg{
		Repo:   repo,
		Online: false,
	}

	ipfsNode, err := core.NewNode(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return ipfsNode, nil
}

func newCctx(repoPath string) *oldcmds.Context {
	return &oldcmds.Context{
		ConfigRoot: repoPath,
		LoadConfig: fsrepo.ConfigAt,
		ReqLog:     &oldcmds.ReqLog{},
	}
}

func startDaemon(ctx context.Context, cctx *oldcmds.Context, c *Config) error {
	repo, err := InitDefaultOrMigrateRepoIfNeeded(c)
	if err != nil {
		return err
	}

	cfg, err := cctx.GetConfig()
	if err != nil {
		return err
	}

	// Start assembling node config
	ncfg := &core.BuildCfg{
		Repo:                        repo,
		Permanent:                   true, // It is temporary way to signify that node is permanent
		Online:                      true,
		DisableEncryptedConnections: false,
		ExtraOpts: map[string]bool{
			"pubsub": c.EnableFloodSub,
			"ipnsps": c.EnableIPNSPubSub,
			"mplex":  c.EnableMultiplex,
		},
		//TODO(Kubuxu): refactor Online vs Offline by adding Permanent vs Ephemeral
	}

	switch cfg.Routing.Type {
	case routingOptionDHTClientKwd:
		ncfg.Routing = core.DHTClientOption
	case routingOptionDHTKwd:
		ncfg.Routing = core.DHTOption
	case routingOptionNoneKwd:
		ncfg.Routing = core.NilRouterOption
	default:
		return fmt.Errorf("unrecognized routing option: %s", cfg.Routing.Type)
	}

	node, err := core.NewNode(ctx, ncfg)
	if err != nil {
		return err
	}

	node.SetLocal(false)
	cctx.ConstructNode = func() (*core.IpfsNode, error) { return node, nil }

	startGC(ctx, node, cfg)
	return nil
}

func startGC(ctx context.Context, node *core.IpfsNode, cfg *config.Config) {
	// ignore if not set
	if cfg.Datastore.GCPeriod == "" {
		return
	}

	go func() {
		err := corerepo.PeriodicGC(ctx, node)
		log.Error("PeriodicGC:", err)
	}()
}
