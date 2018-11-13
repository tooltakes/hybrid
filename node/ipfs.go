package node

import (
	"context"
	"strings"

	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/domain"
	"github.com/empirefox/hybrid/pkg/ipfs"
	"go.uber.org/zap"

	"github.com/ipsn/go-ipfs/core"
)

func NewIpfs(ctx context.Context, c *config.Config, log *zap.Logger) (*ipfs.Ipfs, error) {
	apiListenAddr, err := parseTCPMultiaddr(c.Ipfs.FakeApiListenAddr)
	if err != nil {
		log.Error("Ipfs.FakeApiListenAddr", zap.Error(err))
		return nil, err
	}

	gatewayAddr, err := parseTCPMultiaddr(c.Bind)
	if err != nil {
		log.Error("Config.Bind", zap.Error(err))
		return nil, err
	}

	t, err := c.ConfigTree()
	if err != nil {
		log.Error("ConfigTree", zap.Error(err))
		return nil, err
	}

	ipfsConfig := &ipfs.Config{
		FakeApiListenAddr: apiListenAddr,
		GatewayListenAddr: gatewayAddr,
		ExcludeIPNS:       func(host string) bool { return strings.HasSuffix(host, domain.HybridSuffix) },

		RepoPath:         t.IpfsPath,
		Profile:          c.Ipfs.Profile,
		AutoMigrate:      c.Ipfs.AutoMigrate,
		EnableIPNSPubSub: c.Ipfs.EnableIPNSPubSub,
		EnablePubSub:     c.Ipfs.EnablePubSub,
		EnableMultiplex:  c.Ipfs.EnableMultiplex,
	}
	hi, err := ipfs.NewIpfs(ctx, ipfsConfig)
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
