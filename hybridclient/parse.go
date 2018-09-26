package hybridclient

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridutils"
	"go.uber.org/zap"
)

type tcpServer struct {
	Name         string
	Addr         string
	NoTLS        bool
	ClientScalar *[32]byte
	ServerPubkey *[32]byte
	NoAuth       bool
	Token        []byte
}

func newTcpServer(raw TcpServer, token []byte) (*tcpServer, error) {
	s := tcpServer{
		Name:   raw.Name,
		Addr:   raw.Addr,
		NoTLS:  raw.NoTLS,
		NoAuth: raw.NoAuth,
		Token:  []byte(raw.Token),
	}

	if s.Name == "" {
		s.Name = strings.Replace(s.Addr, ":", "-", -1)
	}
	if len(s.Token) == 0 {
		s.Token = token
	}

	if !s.NoTLS {
		if raw.ClientScalarHex != "" {
			scalar, err := hybridutils.DecodeKey32(raw.ClientScalarHex)
			if err != nil {
				return nil, err
			}
			s.ClientScalar = scalar
		}
		pubkey, err := hybridutils.DecodeKey32(raw.ServerPublicHex)
		if err != nil {
			return nil, err
		}
		s.ServerPubkey = pubkey
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

func (c *Client) newRouter(raw RouterItem) (hybrid.Router, error) {
	if raw.Adp != nil && raw.IPNet == nil {
		return c.newAdpRouter(raw.Name, raw.Adp)
	}
	if raw.Adp == nil && raw.IPNet != nil {
		return c.newNetRouter(raw.Name, raw.IPNet)
	}
	return nil, fmt.Errorf("one and only one router can be set in RouterItem(%s)", raw.Name)
}

func (c *Client) newAdpRouter(name string, raw *AdpRouter) (*hybrid.AdpRouter, error) {
	config := hybrid.AdpRouterConfig{
		Log:                 c.log,
		Disabled:            c.routerDisabled[name],
		EtcHostsIPAsBlocked: raw.EtcHostsIPAsBlocked,
		Dev:                 raw.Dev,
	}

	if raw.Blocked != "" {
		blocked, ok := c.proxies[raw.Blocked]
		if !ok {
			return nil, fmt.Errorf("AdpRouter(%s) blocked name(%s) not found", name, raw.Blocked)
		}
		config.Blocked = blocked
	}
	if raw.Unblocked != "" {
		unblocked, ok := c.proxies[raw.Unblocked]
		if !ok {
			return nil, fmt.Errorf("AdpRouter(%s) unblocked name(%s) not found", name, raw.Unblocked)
		}
		config.Unblocked = unblocked
	}

	if raw.B64RuleDirName != "" {
		b64, err := c.readRulesDir(raw.B64RuleDirName)
		if err != nil {
			return nil, err
		}
		config.B64Rules = b64
	}

	if raw.TxtRuleDirName != "" {
		txt, err := c.readRulesDir(raw.TxtRuleDirName)
		if err != nil {
			return nil, err
		}
		config.TxtRules = txt
	}

	return hybrid.NewAdpRouter(config)
}

func (c *Client) readRulesDir(dirname string) ([][]byte, error) {
	dir := filepath.Join(c.ruleRootDir, dirname)

	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		c.log.Error("readRulesDir", zap.Error(err))
		return nil, err
	}

	var names []string
	for _, info := range infos {
		name := info.Name()
		n := name[0]
		if !info.IsDir() && n >= '0' && n <= '9' {
			names = append(names, name)
		} else {
			c.log.Info("readRulesDir", zap.String("file", name), zap.String("dir", dir))
		}
	}

	sort.Strings(names)
	contents := make([][]byte, 0, len(names))
	for _, name := range names {
		content, err := ioutil.ReadFile(filepath.Join(dir, name))
		if err != nil {
			c.log.Error("readRulesDir", zap.Error(err))
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

func (c *Client) newNetRouter(name string, raw *IPNetRouter) (*hybrid.IPNetRouter, error) {
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
			c.log.Error("IPNetRouter", zap.Error(err))
			return nil, err
		}
		nets[i] = cidr
	}

	router := hybrid.IPNetRouter{
		Skip: c.routerDisabled[name],
		IPs:  ips,
		Nets: nets,
	}

	if raw.Matched != "" {
		p, ok := c.proxies[raw.Matched]
		if !ok {
			return nil, fmt.Errorf("IPNetRouter(%s) matched name(%s) not found", name, raw.Matched)
		}
		router.Matched = p
	}

	if raw.Unmatched != "" {
		p, ok := c.proxies[raw.Unmatched]
		if !ok {
			return nil, fmt.Errorf("IPNetRouter(%s) unmatched name(%s) not found", name, raw.Unmatched)
		}
		router.Unmatched = p
	}

	if raw.FileTest != "" {
		p, ok := c.fileClients[raw.FileTest]
		if !ok {
			return nil, fmt.Errorf("IPNetRouter(%s) FileTest(%s) not found", name, raw.FileTest)
		}
		router.FileClient = p
	}

	return &router, nil
}
