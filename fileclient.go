package hybrid

import (
	"io"
	"io/ioutil"

	"github.com/empirefox/hybrid/hybridzipfs"
	"go.uber.org/zap"
)

type FileClientConfig struct {
	Log      *zap.Logger
	Dev      bool
	Disabled bool
	RootZip  string
	Redirect map[string]string
}

// FileClient implements both Router and Proxy.
type FileClient struct {
	log    *zap.Logger
	config FileClientConfig
	hfs    *hybridzipfs.GzipHttpfs
	io.Closer
}

func NewFileClient(config FileClientConfig) (*FileClient, error) {
	hfs, closer, err := hybridzipfs.New(config.RootZip + ".zip")
	if err != nil {
		config.Log.Error("hybridzipfs.New", zap.String("RootZip", config.RootZip), zap.Error(err))
		return nil, err
	}

	return &FileClient{
		log:    config.Log,
		config: config,
		hfs:    hfs,
		Closer: closer,
	}, nil
}

func (r *FileClient) Route(c *Context) Proxy {
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

func (r *FileClient) Disabled() bool { return r == nil || r.config.Disabled }

func (r *FileClient) Do(c *Context) error {
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

	return c.PipeRoundTrip(r.hfs.RoundTrip)
}

func (p *FileClient) HttpErr(c *Context, code int, info string) {
	he := &HttpErr{
		Code:       code,
		ClientType: "Zip",
		ClientName: p.config.RootZip,
		TargetHost: c.HostPort,
		Info:       info,
	}
	c.HttpErr(he)
}
