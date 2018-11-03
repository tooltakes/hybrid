package hybridproxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"go.uber.org/zap"
	"golang.org/x/net/http2"

	"github.com/empirefox/hybrid/pkg/core"
)

type H2Dialer struct {
	Name string
	Dial func() (c net.Conn, err error)
}

type H2ClientConfig struct {
	Log *zap.Logger
}

type H2Client struct {
	log     *zap.Logger
	config  H2ClientConfig
	dialers map[string]*H2Dialer
	tr      *http2.Transport
}

func NewH2Client(config H2ClientConfig) *H2Client {
	h2 := H2Client{
		log:     config.Log,
		config:  config,
		dialers: make(map[string]*H2Dialer),
		tr:      &http2.Transport{AllowHTTP: true},
	}

	h2.tr.DialTLS = h2.DialAndAuth

	return &h2
}

func (h2 *H2Client) AddDialer(dialer *H2Dialer) (*H2Proxy, error) {
	h2.dialers[dialer.Name] = dialer
	return &H2Proxy{
		client: h2,
		idx:    dialer.Name,
	}, nil
}

func (h2 *H2Client) DialAndAuth(network, addr string, cfg *tls.Config) (net.Conn, error) {
	idx, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer, ok := h2.dialers[idx]
	if !ok {
		return nil, fmt.Errorf("Dialer not found: %s", idx)
	}

	return dialer.Dial()
}

func (h2 *H2Client) Proxy(c *hybridcore.Context, idx string) error {
	req := c.Request
	// keep underline real conn
	req.Close = false
	// fix for http2.checkConnHeaders
	req.Header.Del("Connection")
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if c.Connect {
		req.Body = c.NopCloserBody()
	}

	// Host => authority|target, so the real schema can be ignored
	if req.URL.Scheme == "https" {
		req.Host = string(hybridcore.HostHttpsPrefix) + req.Host
	} else {
		req.Host = string(hybridcore.HostHttpPrefix) + req.Host
	}

	// used for underline conn dial
	req.URL.Scheme = "http"
	req.URL.Host = idx // => for dial, this will not be send because of req.Host

	return c.PipeTransport(h2.tr)
}
