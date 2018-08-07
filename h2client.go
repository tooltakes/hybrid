package hybrid

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type H2Dialer struct {
	Dial func() (c net.Conn, err error)

	NoTLS        bool
	ClientScalar *[32]byte
	ServerPubkey *[32]byte

	NoAuth bool
	Token  []byte

	ccc CryptoConnConfig

	tokenWithLen []byte
}

type H2ClientConfig struct {
	Log          *zap.Logger
	Scalar       *[32]byte
	LocalHandler http.Handler
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
	idx := fmt.Sprintf("%x", len(h2.dialers))
	h2.dialers[idx] = dialer

	if !dialer.NoTLS || !dialer.NoAuth {
		// only do auth standalone when not using tls
		tokenLen := len(dialer.Token)
		if tokenLen > MaxAuthTokenSize || tokenLen == 0 {
			return nil, fmt.Errorf("Token len should be in (0,%d], but is %d", MaxAuthTokenSize, len(dialer.Token))
		}

		dialer.tokenWithLen = make([]byte, tokenLen+1)
		dialer.tokenWithLen[0] = byte(tokenLen)
		copy(dialer.tokenWithLen[1:], dialer.Token)
	}

	if !dialer.NoTLS {
		// do tls will do auth at the same time
		dialer.ccc = CryptoConnConfig{
			ServerPublic:     dialer.ServerPubkey,
			ClientScalar:     dialer.ClientScalar,
			Authorization:    dialer.Token,
			TimestampValidIn: 60,
		}
		if dialer.ccc.ClientScalar == nil {
			dialer.ccc.ClientScalar = h2.config.Scalar
		}
	}

	return &H2Proxy{
		client: h2,
		idx:    idx,
	}, nil
}

func (h2 *H2Client) DialAndAuth(network, addr string, cfg *tls.Config) (c net.Conn, err error) {
	idx, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer, ok := h2.dialers[idx]
	if !ok {
		return nil, fmt.Errorf("Dialer not found: %s", idx)
	}

	c, err = h2.dialTLS(dialer, idx)
	if err != nil {
		return
	}

	if !dialer.NoTLS || dialer.NoAuth {
		return
	}

	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	_, err = c.Write(dialer.tokenWithLen)
	if err != nil {
		return
	}

	var b [1]byte
	_, err = c.Read(b[:])
	if err != nil {
		return
	}
	if b[0] != ClientAuthOK {
		err = os.ErrPermission
		return
	}

	//	go h2.ping(c, addr)
	return
}

func (h2 *H2Client) dialTLS(dialer *H2Dialer, idx string) (net.Conn, error) {
	rawConn, err := dialer.Dial()
	if err != nil {
		return nil, err
	}
	if dialer.NoTLS {
		return rawConn, nil
	}

	c, err := NewCryptoConn(rawConn, &dialer.ccc)
	if err != nil {
		rawConn.Close()
	}
	return c, err
}

func (h2 *H2Client) ping(conn io.Closer, addr string) {
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

func (h2 *H2Client) Proxy(c *Context, idx string) {
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
	if req.Host == HostLocalServer {
		// Host already set by Hybrid
	} else if req.URL.Scheme == "https" {
		req.Host = string(HostHttpsPrefix) + c.HostPort
	} else {
		req.Host = string(HostHttpPrefix) + c.HostPort
	}

	// used for underline conn dial
	req.URL.Scheme = "http"
	req.URL.Host = idx // => for dial, this will not be send because of req.Host

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)
	res, err := h2.tr.RoundTrip(req)
	if err != nil {
		h2.log.Warn(c.HostPort, zap.Error(err))
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
