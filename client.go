package hybrid

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type Dialer interface {
	Dial(addr string) (c net.Conn, serverNames []string, err error)
}

type ClientConfig struct {
	Log             *zap.Logger
	Dialer          Dialer
	NoTLS           bool
	TLSClientConfig *tls.Config
	LocalHandler    http.Handler
}

type Client struct {
	log          *zap.Logger
	notls        bool
	dialer       Dialer
	localHandler http.Handler
	tr           *http2.Transport
}

func NewClient(config ClientConfig) *Client {
	c := Client{
		log:          config.Log,
		notls:        config.NoTLS,
		dialer:       config.Dialer,
		localHandler: config.LocalHandler,
		tr: &http2.Transport{
			AllowHTTP:       config.NoTLS,
			TLSClientConfig: config.TLSClientConfig,
		},
	}

	c.tr.DialTLS = c.Dial

	return &c
}

func (h2 *Client) Dial(network, addr string, cfg *tls.Config) (c net.Conn, err error) {
	c, err = h2.dial(network, addr, cfg)
	if err != nil {
		return
	}
	//	go h2.ping(c, addr)
	return
}

func (h2 *Client) dial(network, addr string, cfg *tls.Config) (net.Conn, error) {
	c, serverNames, err := h2.dialer.Dial(addr)
	if err != nil {
		return nil, err
	}
	if h2.notls {
		return c, nil
	}

	cn := tls.Client(c, cfg)
	if err = HandshakeTLS(cn, serverNames, cfg); err != nil {
		return nil, err
	}
	return cn, nil
}

func HandshakeTLS(cn *tls.Conn, serverNames []string, cfg *tls.Config) error {
	err := cn.Handshake()
	if err != nil {
		return err
	}
	if !cfg.InsecureSkipVerify {
		for _, serverName := range serverNames {
			if err = cn.VerifyHostname(serverName); err == nil {
				break
			}
		}
		if err != nil {
			return err
		}
	}
	state := cn.ConnectionState()
	if p := state.NegotiatedProtocol; p != http2.NextProtoTLS {
		return fmt.Errorf("http2: unexpected ALPN protocol %q; want %q", p, http2.NextProtoTLS)
	}
	if !state.NegotiatedProtocolIsMutual {
		return errors.New("http2: could not negotiate protocol mutually")
	}
	return nil
}

func (h2 *Client) ping(conn io.Closer, addr string) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close()
		h2.log.Info("conn closed")
	}()
	h2.log.Info("conn started")

	req := &http.Request{
		Method: "HEAD",
		URL: &url.URL{
			Scheme: "https",
			Host:   addr,
		},
		Header: make(http.Header),
		Host:   HostPing,
	}

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			res, err := h2.tr.RoundTrip(req.WithContext(ctx))
			if err != nil || res.StatusCode != http.StatusOK {
				cancel()
				return
			}
			cancel()
		}
	}
}

func (h2 *Client) Proxy(c *Context, proxy *url.URL) {
	if h2 == nil {
		c.Writer.Write([]byte("HTTP/1.1 502 H2 Not Found\r\n\r\n"))
		return
	}

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
	if strings.HasSuffix(c.HostNoPort, HostHybridSuffix) {
		if h2.localHandler != nil && (c.HostNoPort == HostLocalhost || c.HostNoPort == HostLocal127) {
			// always close conn after local serve
			h2.localHandler.ServeHTTP(NewResponseWriter(c.Writer), req)
			return
		}
		req.Host = HostLocalServer
	} else if req.URL.Scheme == "https" {
		req.Host = string(HostHttpsPrefix) + c.HostPort
	} else {
		req.Host = string(HostHttpPrefix) + c.HostPort
	}

	// used for underline conn dial
	if h2.notls {
		req.URL.Scheme = "http"
	} else {
		req.URL.Scheme = "https"
	}
	req.URL.Host = proxy.Host // => for dial, this will not be send because of req.Host

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)
	res, err := h2.tr.RoundTrip(req)
	if err != nil {
		h2.log.Error(c.HostPort, zap.Error(err))
		c.Writer.Write([]byte("HTTP/1.1 502 No Trip\r\n\r\n"))
		return
	}
	defer res.Body.Close()

	if c.Connect {
		if res.StatusCode != http.StatusOK {
			c.Writer.Write([]byte(fmt.Sprintf("HTTP/1.1 %d Server Fail\r\n\r\n", res.StatusCode)))
			return
		}
		c.Writer.Write(StandardConnectOK)
		io.Copy(c.Writer, res.Body)
	} else {
		res.Close = true
		res.Write(c.Writer)
		// close any h2 reverse request
	}
}
