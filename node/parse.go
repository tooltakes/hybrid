package node

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/core"
	"github.com/empirefox/hybrid/pkg/proxy"
	"go.uber.org/zap"

	peer "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peer"
	ma "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr"
	manet "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr-net"
)

type ipfsServer struct {
	Name     string
	Peer     peer.ID
	Protocol string
	Token    []byte
}

func newIpfsServer(raw *config.IpfsServer, token []byte) (*ipfsServer, error) {
	id, err := peer.IDB58Decode(raw.Peer)
	if err != nil {
		return nil, err
	}

	s := ipfsServer{
		Name:  raw.Name,
		Peer:  id,
		Token: []byte(raw.Token),
	}
	if s.Name == "" {
		s.Name = raw.Peer
	}
	if len(s.Token) == 0 {
		s.Token = token
	}
	return &s, nil
}

func parseEnvList(name string) map[string]bool {
	list := make(map[string]bool)
	for _, name := range strings.Split(os.Getenv(name), ",") {
		if name != "" {
			list[name] = true
		}
	}
	return list
}

func (n *Node) newRouter(raw *config.RouterItem) (core.Router, error) {
	if router := raw.GetAdp(); router != nil {
		return n.newAdpRouter(raw.Name, router)
	}
	if router := raw.GetIpnet(); router != nil {
		return n.newNetRouter(raw.Name, router)
	}
	return nil, fmt.Errorf("one and only one router can be set in RouterItem(%s)", raw.Name)
}

func (n *Node) newAdpRouter(name string, raw *config.AdpRouter) (*proxy.AdpRouter, error) {
	config := proxy.AdpRouterConfig{
		Log:                 n.log,
		Disabled:            n.routerDisabled[name],
		EtcHostsIpAsBlocked: raw.EtcHostsIpAsBlocked,
		Dev:                 raw.Dev,
	}

	if raw.Blocked != "" {
		blocked, ok := n.proxies[raw.Blocked]
		if !ok {
			return nil, fmt.Errorf("AdpRouter(%s) blocked name(%s) not found", name, raw.Blocked)
		}
		config.Blocked = blocked
	}
	if raw.Unblocked != "" {
		unblocked, ok := n.proxies[raw.Unblocked]
		if !ok {
			return nil, fmt.Errorf("AdpRouter(%s) unblocked name(%s) not found", name, raw.Unblocked)
		}
		config.Unblocked = unblocked
	}

	rcs, err := n.openRulesDir(raw.RulesDirName)
	if err != nil {
		return nil, err
	}
	config.Rules = rcs

	return proxy.NewAdpRouter(config)
}

func (n *Node) openRulesDir(dirname string) ([]io.ReadCloser, error) {
	dir := filepath.Join(n.ruleRootDir, dirname)

	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		n.log.Error("readRulesDir", zap.Error(err))
		return nil, err
	}

	var names []string
	for _, info := range infos {
		name := info.Name()
		n0 := name[0]
		if !info.IsDir() && n0 >= '0' && n0 <= '9' {
			names = append(names, name)
		} else {
			n.log.Info("readRulesDir", zap.String("file", name), zap.String("dir", dir))
		}
	}

	sort.Strings(names)
	result := make([]io.ReadCloser, 0, 32)
	for _, name := range names {
		rcs, err := n.OpenIntentFile(dir, name)
		if err != nil {
			for _, rc := range rcs {
				rc.Close()
			}
			return nil, err
		}
		result = append(result, rcs...)
	}
	return result, nil
}

type netRouter struct {
	Ip        []string `validate:"dive,ip"`
	Net       []string `validate:"dive,cidr"`
	Matched   string   `validate:"required"`
	Unmatched string   `validate:"required,nefield=Matched"`
	FileTest  string
}

func (n *Node) newNetRouter(name string, raw *config.IPNetRouter) (*proxy.IPNetRouter, error) {
	ips := make([]net.IP, len(raw.Ip))
	for i, ipr := range raw.Ip {
		ip := net.ParseIP(ipr)
		if ip == nil {
			return nil, fmt.Errorf("%s is not a ip in router(%s)", ipr, name)
		}
		ips[i] = ip
	}

	nets := make([]*net.IPNet, len(raw.Net))
	for i, netr := range raw.Net {
		_, cidr, err := net.ParseCIDR(netr)
		if err != nil {
			n.log.Error("IPNetRouter", zap.Error(err))
			return nil, err
		}
		nets[i] = cidr
	}

	router := proxy.IPNetRouter{
		Skip: n.routerDisabled[name],
		IPs:  ips,
		Nets: nets,
	}

	if raw.Matched != "" {
		p, ok := n.proxies[raw.Matched]
		if !ok {
			return nil, fmt.Errorf("IPNetRouter(%s) matched name(%s) not found", name, raw.Matched)
		}
		router.Matched = p
	}

	if raw.Unmatched != "" {
		p, ok := n.proxies[raw.Unmatched]
		if !ok {
			return nil, fmt.Errorf("IPNetRouter(%s) unmatched name(%s) not found", name, raw.Unmatched)
		}
		router.Unmatched = p
	}

	if raw.FileTest != "" {
		p, ok := n.fileClients[raw.FileTest]
		if !ok {
			return nil, fmt.Errorf("IPNetRouter(%s) FileTest(%s) not found", name, raw.FileTest)
		}
		router.FileClient = p
	}

	return &router, nil
}

func parseTCPMultiaddr(addr string) (ma.Multiaddr, error) {
	naddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	if naddr.IP == nil {
		naddr.IP = net.IPv4zero
	}
	return manet.FromNetAddr(naddr)
}
