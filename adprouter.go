package hybrid

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/cbednarski/hostess"
	"github.com/pmezard/adblock/adblock"
)

type AdpRouterConfig struct {
	Log       *zap.Logger
	Disabled  bool
	Exist     string
	Blocked   *url.URL
	Unblocked *url.URL
	B64Rules  [][]byte
	TxtRules  [][]byte
}

type AdpRouter struct {
	log          *zap.Logger
	config       *AdpRouterConfig
	adpMatcher   *adblock.RuleMatcher
	blockedIps   map[string]bool
	blockedIpsMu sync.RWMutex
}

func NewAdpRouter(config *AdpRouterConfig) (*AdpRouter, error) {
	r := &AdpRouter{
		log:        config.Log,
		config:     config,
		adpMatcher: adblock.NewMatcher(),
		blockedIps: make(map[string]bool),
	}
	err := r.init()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *AdpRouter) Route(c *Context) *url.URL {
	if blocked := r.blocked(c); blocked {
		return r.config.Blocked
	}
	return r.config.Unblocked
}

func (r *AdpRouter) blocked(c *Context) bool {
	r.blockedIpsMu.RLock()
	if r.blockedIps[c.HostNoPort] {
		r.blockedIpsMu.RUnlock()
		return true
	}
	r.blockedIpsMu.RUnlock()

	blocked := r.AdpMatch("http://" + c.HostNoPort)
	if blocked {
		r.blockedIpsMu.Lock()
		r.blockedIps[c.HostNoPort] = true
		r.blockedIpsMu.Unlock()
	}
	return blocked
}

func (r *AdpRouter) init() error {
	var added int
	for _, b := range r.config.TxtRules {
		n, err := LoadAbpRules(r.adpMatcher, b, false)
		if err != nil {
			r.log.Error("LoadAbpRules", zap.Error(err))
		}
		added += n
	}
	for _, b := range r.config.B64Rules {
		n, err := LoadAbpRules(r.adpMatcher, b, true)
		if err != nil {
			r.log.Error("LoadAbpRules", zap.Error(err))
		}
		added += n
	}

	r.log.Info("AdpList rules loaded", zap.Int("total", added))

	// init blockedIps only after adpMatcher
	hf, errs := hostess.LoadHostfile()
	if errs != nil {
		r.log.Debug("hosts errors")
		for _, err := range errs {
			r.log.Debug("hosts entry", zap.Error(err))
		}
	}

	for _, hostname := range hf.Hosts {
		if hostname.Enabled && hostname.IsValid() && r.AdpMatch("http://"+hostname.Domain) {
			r.blockedIps[hostname.IP.String()] = true
		}
	}

	return nil
}

func (r *AdpRouter) Disabled() bool { return r.config.Disabled }
func (r *AdpRouter) Exist() string  { return r.config.Exist }

func (r *AdpRouter) AdpMatch(u string) bool {
	rq := &adblock.Request{
		URL:          u,
		Domain:       "",
		OriginDomain: "",
		ContentType:  "",
		Timeout:      5 * time.Second,
	}
	matched, _, err := r.adpMatcher.Match(rq)
	if err != nil {
		r.log.Error("AdpMatch", zap.Error(err))
		return false // No Block here
	}

	return matched
}

// just copy
func LoadAbpRules(m *adblock.RuleMatcher, b []byte, b64 bool) (int, error) {
	var r io.Reader = bytes.NewReader(b)
	if b64 {
		r = base64.NewDecoder(base64.StdEncoding, r)
	}
	parsed, err := adblock.ParseRules(r)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, rule := range parsed {
		err := m.AddRule(rule, 0)
		if err == nil {
			added += 1
		}
	}
	return added, nil
}
