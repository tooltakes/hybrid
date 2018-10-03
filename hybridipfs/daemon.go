package hybridipfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/plugin/loader"
	oldcmds "github.com/ipsn/go-ipfs/commands"
	"github.com/ipsn/go-ipfs/core"
	core "github.com/ipsn/go-ipfs/core"
	"github.com/ipsn/go-ipfs/core/corehttp"
	corehttp "github.com/ipsn/go-ipfs/core/corehttp"
	"github.com/ipsn/go-ipfs/core/corerepo"
	corerepo "github.com/ipsn/go-ipfs/core/corerepo"
	fsrepo "github.com/ipsn/go-ipfs/repo/fsrepo"
	"github.com/qiniu/log"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-config"
	ma "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr-net"
)

const (
	routingOptionDHTClientKwd = "dhtclient"
	routingOptionDHTKwd       = "dht"
	routingOptionNoneKwd      = "none"
)

var (
	// ErrIsOffline is returned when an online operation was done offline.
	ErrIsOffline = errors.New("Node is offline")
	ErrIsOnline  = errors.New("Node is online")
)

type DaemonOptions struct {
	RepoPath         string
	Profile          string
	AutoMigrate      bool
	EnableIPNSPubSub bool
	EnableFloodSub   bool
	EnableMultiplex  bool
}

// Node remembers the settings needed for accessing the ipfs daemon.
type Node struct {
	DaemonOptions DaemonOptions

	ctx           context.Context
	apiListenAddr ma.Multiaddr
	apiDialAddr   ma.Multiaddr

	mu        sync.Mutex
	ipfsNode  *core.IpfsNode
	apiDialer Dialer
	cancel    context.CancelFunc
}

func NewNode(ctx context.Context, apiListenAddr, apiDialAddr ma.Multiaddr, opts DaemonOptions) (*Node, error) {
	if opts.RepoPath == "" {
		repoPath, err := fsrepo.BestKnownPath()
		if err != nil {
			return nil, err
		}
		opts.RepoPath = repoPath
	}

	err = CheckPlugins(opts.RepoPath)
	if err != nil {
		return nil, err
	}

	ipfsNode, err := createOfflineNode(ctx, opts.RepoPath)
	if err != nil {
		return nil, err
	}

	return &Node{
		DaemonOptions: DaemonOptions,

		ctx:           ctx,
		apiListenAddr: apiListenAddr,
		apiDialAddr:   apiDialAddr,

		ipfsNode: ipfsNode,
	}, nil
}

func (nd *Node) IsOnline() bool {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.isOnline()
}

func (nd *Node) isOnline() bool {
	return nd.ipfsNode.OnlineMode()
}

func (nd *Node) Connect() error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	if nd.isOnline() {
		return nil
	}

	cctx := newCctx(nd.DaemonOptions.RepoPath)
	ln, apiDialer := NewListenDialer(nd.apiListenAddr, nd.apiDialAddr)
	ctx, cancel := context.WithCancel(context.Background())
	var onErr = func() {
		cancel()
		ln.Close()
		cctx.Close()
	}
	errc, err := startDaemon(ctx, cctx, ln, &nd.DaemonOptions)
	if err != nil {
		onErr()
		return err
	}

	ipfsNode, err := cctx.GetNode()
	if err != nil {
		onErr()
		return err
	}

	if nd.ipfsNode != nil {
		nd.ipfsNode.Close()
	}
	nd.ipfsNode = ipfsNode
	nd.apiDialer = apiDialer
	nd.cancel = cancel
	return nil
}

func (nd *Node) Disconnect() (err error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	if !nd.isOnline() {
		return ErrIsOffline
	}

	nd.cancel()
	nd.ipfsNode.Close()
	nd.ipfsNode, err = createOfflineNode(nd.ctx, nd.DaemonOptions.RepoPath)
	return
}

func (nd *Node) Name() string {
	return "ipfs"
}

func CheckPlugins(repoPath string) error {
	// check if repo is accessible before loading plugins
	ok, err := checkPermissions(repoPath)
	if err != nil {
		return err
	}
	if ok {
		pluginpath := filepath.Join(repoPath, "plugins")
		if _, err := loader.LoadPlugins(pluginpath); err != nil {
			log.Error("error loading plugins: ", err)
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

func createOfflineNode(ctx context.Context, repoPath string) (*core.IpfsNode, error) {
	repo, err := InitDefaultOrMigrateRepoIfNeeded(repoPath, "")
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

func startDaemon(ctx context.Context, cctx *oldcmds.Context, ln manet.Listener, opts *DaemonOptions) (<-chan error, error) {
	repo, err := InitDefaultOrMigrateRepoIfNeeded(cctx.ConfigRoot, opts.Profile)
	if err != nil {
		return nil, err
	}

	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, err
	}

	// Start assembling node config
	ncfg := &core.BuildCfg{
		Repo:                        repo,
		Permanent:                   true, // It is temporary way to signify that node is permanent
		Online:                      true,
		DisableEncryptedConnections: false,
		ExtraOpts: map[string]bool{
			"pubsub": opts.EnableFloodSub,
			"ipnsps": opts.EnableIPNSPubSub,
			"mplex":  opts.EnableMultiplex,
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
		return nil, fmt.Errorf("unrecognized routing option: %s", routingOption)
	}

	node, err := core.NewNode(ctx, ncfg)
	if err != nil {
		return nil, err
	}

	node.SetLocal(false)
	cctx.ConstructNode = func() (*core.IpfsNode, error) { return node, nil }

	// construct api endpoint - every time
	apiErrc, err := serveHTTPApi(node, cctx, cfg, ln)
	if err != nil {
		node.Close()
		return nil, err
	}

	// construct http gateway - if it is set in the config
	var gwErrc <-chan error
	if len(cfg.Addresses.Gateway) > 0 {
		var err error
		gwErrc, err = serveHTTPGateway(node, cctx, cfg)
		if err != nil {
			node.Close()
			return nil, err
		}
	}

	gcErrc := startGC(ctx, node, cfg)
	merged := merge(apiErrc, gwErrc, gcErrc)
	return oneFromMerged(merged), nil
}

func serveHTTPApi(node *core.IpfsNode, cctx *oldcmds.Context, cfg *config.Config, ln manet.Listener) (<-chan error, error) {
	var opts = []corehttp.ServeOption{
		corehttp.CommandsOption(*cctx),
		corehttp.WebUIOption,
		corehttp.GatewayOption(true, "/ipfs", "/ipns"),
		corehttp.LogOption(),
	}
	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	errc := make(chan error)
	go func() {
		errc <- corehttp.Serve(node, manet.NetListener(ln), opts...)
		close(errc)
	}()
	return errc, nil
}

func serveHTTPGateway(node *core.IpfsNode, cctx *oldcmds.Context, cfg *config.Config) (<-chan error, error) {
	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGateway: invalid gateway address: %q (err: %s)", cfg.Addresses.Gateway, err)
	}

	gwLis, err := manet.Listen(gatewayMaddr)
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
	}

	var opts = []corehttp.ServeOption{
		corehttp.IPNSHostnameOption(),
		corehttp.GatewayOption(cfg.Gateway.Writable, "/ipfs", "/ipns"),
		corehttp.CheckVersionOption(),
		corehttp.CommandsROOption(*cctx),
	}
	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	errc := make(chan error)
	go func() {
		errc <- corehttp.Serve(node, manet.NetListener(gwLis), opts...)
		close(errc)
	}()
	return errc, nil
}

func startGC(ctx context.Context, node *core.IpfsNode, cfg *config.Config) <-chan error {
	// ignore if not set
	if cfg.Datastore.GCPeriod == "" {
		return nil
	}

	errc := make(chan error)
	go func() {
		errc <- corerepo.PeriodicGC(ctx, node)
		close(errc)
	}()
	return errc, nil
}

// merge does fan-in of multiple read-only error channels
// taken from http://blog.golang.org/pipelines
func merge(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan error) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	for _, c := range cs {
		if c != nil {
			wg.Add(1)
			go output(c)
		}
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func oneFromMerged(es <-chan error) <-chan error {
	out := make(chan error)
	go func() {
		var errs []string
		for e := range es {
			if e != nil {
				errs = append(errs, e.Error())
			}
		}
		out <- errors.New(strings.Join(errs, "\n"))
		close(out)
	}()
	return out
}
