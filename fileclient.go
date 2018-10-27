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

func (r *FileClient) Do(c *Context) {
	req := c.Request

	if r.config.Redirect != nil {
		newPath, ok := r.config.Redirect[c.Request.URL.Path]
		if ok {
			// localRedirect gives a Moved Permanently response.
			// It does not convert relative paths to absolute paths like Redirect does.
			if q := req.URL.RawQuery; q != "" {
				newPath += "?" + q
			}
			newPath += "\r\n"
			c.Writer.Write(append(Standard301Prefix, []byte(newPath)...))
			return
		}
	}

	// TODO another way to avoid fs.go redirect?
	if req.URL.Path == "/index.html" {
		req.URL.Path = "/"
	}
	if req.Body != nil {
		req.Body = ioutil.NopCloser(req.Body)
	}
	res, err := r.hfs.RoundTrip(req)
	if err != nil {
		r.log.Error("Failed to serve local CDN file", zap.Error(err), zap.Stringer("url", c.Request.URL))
		c.Writer.Write(Standard502LocalCDN)
		return
	}
	err = res.Write(c.Writer)
	if err != nil {
		r.log.Error("Failed to write local CDN file to conn", zap.Error(err), zap.Stringer("url", c.Request.URL))
	}
}
