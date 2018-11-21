package proxy

import (
	"io"
	"io/ioutil"

	"go.uber.org/zap"

	"github.com/empirefox/hybrid/pkg/core"
	"github.com/empirefox/hybrid/pkg/zipfs"
)

var (
	Standard301Prefix = []byte("HTTP/1.1 301 Moved Permanently\r\nLocation: ")
)

type FileClientConfig struct {
	Log      *zap.Logger
	Dev      bool
	Disabled bool
	Zip      string
	Redirect map[string]string
}

// FileProxyRouterClient implements both Router and Proxy.
type FileProxyRouterClient struct {
	log    *zap.Logger
	config FileClientConfig
	hfs    *zipfs.GzipHttpfs
	io.Closer
}

func NewFileProxyRouterClient(config FileClientConfig) (*FileProxyRouterClient, error) {
	hfs, closer, err := zipfs.New(config.Zip + ".zip")
	if err != nil {
		config.Log.Error("zipfs.New", zap.String("Zip", config.Zip), zap.Error(err))
		return nil, err
	}

	return &FileProxyRouterClient{
		log:    config.Log,
		config: config,
		hfs:    hfs,
		Closer: closer,
	}, nil
}

// Route implements Router
func (r *FileProxyRouterClient) Route(c *core.Context) core.Proxy {
	if r == nil {
		return nil
	}
	if !c.Connect && r.hfs.CanRequest(c.Request.URL.Path) {
		if r.config.Dev {
			r.log.Debug("Local Y", zap.String("host", c.Request.URL.Host), zap.String("path", c.Request.URL.Path))
		}
		return r
	}
	if r.config.Redirect != nil {
		if _, ok := r.config.Redirect[c.Request.URL.Path]; ok {
			r.log.Debug("Local R", zap.String("host", c.Request.URL.Host), zap.String("path", c.Request.URL.Path))
			return r
		}
	}
	if r.config.Dev {
		r.log.Debug("Local N", zap.String("host", c.Request.URL.Host), zap.String("path", c.Request.URL.Path))
	}
	return nil
}

// Disabled implements Router
func (r *FileProxyRouterClient) Disabled() bool { return r == nil || r.config.Disabled }

// Do implements Proxy
func (r *FileProxyRouterClient) Do(c *core.Context) error {
	req := c.Request

	if r.config.Redirect != nil {
		newPath, ok := r.config.Redirect[req.URL.Path]
		if ok {
			// localRedirect gives a Moved Permanently response.
			// It does not convert relative paths to absolute paths like Redirect does.
			if q := req.URL.RawQuery; q != "" {
				newPath += "?" + q
			}
			newPath += "\r\n\r\n"
			c.Writer.Write(append(Standard301Prefix, []byte(newPath)...))
			return nil
		}
	}

	// TODO another way to avoid fs.go redirect?
	if req.URL.Path == "/index.html" {
		req.URL.Path = "/"
	}
	if req.Body != nil {
		req.Body = ioutil.NopCloser(req.Body)
	}

	return c.PipeTransport(r.hfs)
}

// HttpErr implements Proxy
func (p *FileProxyRouterClient) HttpErr(c *core.Context, code int, info string) {
	he := &core.HttpErr{
		Code:       code,
		ClientType: "Zip",
		ClientName: p.config.Zip,
		TargetHost: c.HostPort,
		Info:       info,
	}
	c.HttpErr(he)
}
