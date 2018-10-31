package hybrid

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
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
	tr      http2.Transport
}

func NewH2Client(config H2ClientConfig) *H2Client {
	h2 := H2Client{
		log:     config.Log,
		config:  config,
		dialers: make(map[string]*H2Dialer),
		tr:      http2.Transport{AllowHTTP: true},
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

func (h2 *H2Client) Proxy(c *Context, idx string) error {
	req := c.Request
	// keep underline conn
	req.Close = false
	// fix for http2.checkConnHeaders
	req.Header.Del("Connection")
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if c.Connect {
		req.ContentLength = -1
		if req.Body == http.NoBody {
			req.Body = ioutil.NopCloser(c.UnsafeReader)
		} else {
			req.Body = ioutil.NopCloser(req.Body)
		}
	} else {
		// need all to be NopCloser, or RoundTrip will not return
		req.Body = ioutil.NopCloser(req.Body)
	}

	// Host => authority|target, so the real schema can be ignored
	if req.URL.Scheme == "https" {
		req.Host = string(HostHttpsPrefix) + c.HostPort
	} else {
		req.Host = string(HostHttpPrefix) + c.HostPort
	}

	// used for underline conn dial
	req.URL.Scheme = "http"
	req.URL.Host = idx // => for dial, this will not be send because of req.Host

	return c.PipeRoundTrip(h2.tr.RoundTrip)
}
