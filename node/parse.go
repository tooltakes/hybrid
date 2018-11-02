package hybridnode

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/proxy"
	"github.com/empirefox/hybrid/pkg/core"
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

func newIpfsServer(raw hybridconfig.IpfsServer, token []byte) (*ipfsServer, error) {
	id, err := peer.IDB58Decode(raw.Peer)
	if err != nil {
		return nil, err
	}

	s := ipfsServer{
		Name:     raw.Name,
		Peer:     id,
		Protocol: raw.Protocol,
		Token:    []byte(raw.Token),
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

func (n *Node) newRouter(raw hybridconfig.RouterItem) (hybridcore.Router, error) {
	if raw.Adp != nil && raw.IPNet == nil {
		return n.newAdpRouter(raw.Name, raw.Adp)
	}
	if raw.Adp == nil && raw.IPNet != nil {
		return n.newNetRouter(raw.Name, raw.IPNet)
	}
	return nil, fmt.Errorf("one and only one router can be set in RouterItem(%s)", raw.Name)
}

func (n *Node) newAdpRouter(name string, raw *hybridconfig.AdpRouter) (*hybridproxy.AdpRouter, error) {
	config := hybridproxy.AdpRouterConfig{
		Log:                 n.log,
		Disabled:            n.routerDisabled[name],
		EtcHostsIPAsBlocked: raw.EtcHostsIPAsBlocked,
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

	if raw.B64RuleDirName != "" {
		b64, err := n.readRulesDir(raw.B64RuleDirName)
		if err != nil {
			return nil, err
		}
		config.B64Rules = b64
	}

	if raw.TxtRuleDirName != "" {
		txt, err := n.readRulesDir(raw.TxtRuleDirName)
		if err != nil {
			return nil, err
		}
		config.TxtRules = txt
	}

	return hybridproxy.NewAdpRouter(config)
}

func (n *Node) readRulesDir(dirname string) ([][]byte, error) {
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
	contents := make([][]byte, 0, len(names))
	for _, name := range names {
		content, err := ioutil.ReadFile(filepath.Join(dir, name))
		if err != nil {
			n.log.Error("readRulesDir", zap.Error(err))
			return nil, err
		}
		contents = append(contents, content)
	}
	return contents, nil
}

type netRouter struct {
	IPs       []string `validate:"dive,ip"`
	Nets      []string `validate:"dive,cidr"`
	Matched   string   `validate:"required"`
	Unmatched string   `validate:"required,nefield=Matched"`
	FileTest  string
}

func (n *Node) newNetRouter(name string, raw *hybridconfig.IPNetRouter) (*hybridproxy.IPNetRouter, error) {
	ips := make([]net.IP, len(raw.IPs))
	for i, ipr := range raw.IPs {
		ip := net.ParseIP(ipr)
		if ip == nil {
			return nil, fmt.Errorf("%s is not a ip in router(%s)", ipr, name)
		}
		ips[i] = ip
	}

	nets := make([]*net.IPNet, len(raw.Nets))
	for i, netr := range raw.Nets {
		_, cidr, err := net.ParseCIDR(netr)
		if err != nil {
			n.log.Error("IPNetRouter", zap.Error(err))
			return nil, err
		}
		nets[i] = cidr
	}

	router := hybridproxy.IPNetRouter{
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
