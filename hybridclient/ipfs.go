package hybridclient

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridipfs"
	"go.uber.org/zap"

	"github.com/ipsn/go-ipfs/core"
	inet "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-net"
)

const PathTokenPrefix = "/token/"

type VerifyFunc func(peerID, token []byte) bool

func NewIpfs(ctx context.Context, config *Config, verify VerifyFunc, log *zap.Logger) (*hybridipfs.Ipfs, error) {
	apiListenAddr, err := parseTCPMultiaddr(config.Ipfs.FakeApiListenAddr)
	if err != nil {
		log.Error("Ipfs.FakeApiListenAddr", zap.Error(err))
		return nil, err
	}

	gatewayAddr, err := parseTCPMultiaddr(config.Bind)
	if err != nil {
		log.Error("Config.Bind", zap.Error(err))
		return nil, err
	}

	t, err := config.ConfigTree()
	if err != nil {
		log.Error("ConfigTree", zap.Error(err))
		return nil, err
	}

	ipfsConfig := &hybridipfs.Config{
		FakeApiListenAddr: apiListenAddr,
		GatewayListenAddr: gatewayAddr,
		ExcludeIPNS:       func(host string) bool { return strings.HasSuffix(host, hybrid.HostHybridSuffix) },

		RepoPath:         t.IpfsPath,
		Profile:          config.Ipfs.Profile,
		AutoMigrate:      config.Ipfs.AutoMigrate,
		EnableIPNSPubSub: config.Ipfs.EnableIPNSPubSub,
		EnableFloodSub:   config.Ipfs.EnableFloodSub,
		EnableMultiplex:  config.Ipfs.EnableMultiplex,
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	hi, err := hybridipfs.NewIpfs(ctx, ipfsConfig)
	if err != nil {
		log.Error("NewNode", zap.Error(err))
		return nil, err
	}

	hi.Connected(func(ipfsNode *core.IpfsNode) {
		log.Debug("IPFS Connected")
	})
	hi.Disconnected(func(ipfsNode *core.IpfsNode) {
		log.Debug("IPFS Disconnected")
	})

	listeners := make([]*hybridipfs.Listener, 0, len(config.Ipfs.ListenProtocols))
	for _, p := range config.Ipfs.ListenProtocols {
		// /hybrid/1.0/token/xxx
		tokenPrefix := HybridIpfsProtocol + PathTokenPrefix
		match := func(protocol string) bool { return strings.HasPrefix(protocol, tokenPrefix) }
		// TODO what if p!=HybridIpfsProtocol
		ln, err := hi.Listen(p, match)
		if err != nil {
			cancel()
			return nil, err
		}

		ln.SetVerify(func(is inet.Stream) bool {
			target := []byte(is.Conn().RemotePeer().Pretty())
			token := []byte(strings.TrimPrefix(string(is.Protocol()), tokenPrefix))
			return verify(target, token)
		})
		listeners = append(listeners, ln)
	}

	s := &hybrid.H2Server{
		Log: log,
		ReverseProxy: &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				log.Debug("Accept request", zap.String("host", req.Host))
			},
			FlushInterval: time.Second,
		},
	}
	for _, ln := range listeners {
		go s.Serve(ln)
	}
	return hi, nil
}
