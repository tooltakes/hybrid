package hybridnode

import (
	"context"
	"strings"

	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/domain"
	"github.com/empirefox/hybrid/pkg/ipfs"
	"go.uber.org/zap"

	"github.com/ipsn/go-ipfs/core"
)

func NewIpfs(ctx context.Context, config *hybridconfig.Config, log *zap.Logger) (*hybridipfs.Ipfs, error) {
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
		ExcludeIPNS:       func(host string) bool { return strings.HasSuffix(host, hybriddomain.HybridSuffix) },

		RepoPath:         t.IpfsPath,
		Profile:          config.Ipfs.Profile,
		AutoMigrate:      config.Ipfs.AutoMigrate,
		EnableIPNSPubSub: config.Ipfs.EnableIPNSPubSub,
		EnableFloodSub:   config.Ipfs.EnableFloodSub,
		EnableMultiplex:  config.Ipfs.EnableMultiplex,
	}
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

	return hi, nil
}
