package hybrid

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

type FileClientConfig struct {
	Log      *zap.Logger
	Disabled bool
	DirPath  string
}

type FileClient struct {
	log          *zap.Logger
	config       FileClientConfig
	roundTripper http.RoundTripper
	pathMap      map[string]bool
}

func NewFileClient(config FileClientConfig) (*FileClient, error) {
	pathMap := make(map[string]bool)
	dirPathSize := len(config.DirPath)
	err := filepath.Walk(config.DirPath+"/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			config.Log.Error("walk dir failed", zap.Error(err), zap.String("path", path))
			return err
		}
		if !info.IsDir() {
			pathMap[path[dirPathSize:]] = true
		}
		return nil
	})
	if err != nil {
		config.Log.Error("walk dir failed", zap.Error(err))
		return nil, err
	}

	if pathMap["/index.html"] {
		pathMap["/"] = true
	}

	return &FileClient{
		log:          config.Log,
		config:       config,
		roundTripper: http.NewFileTransport(http.Dir(config.DirPath)),
		pathMap:      pathMap,
	}, nil
}

func (r *FileClient) Route(c *Context) Proxy {
	if r == nil || c.Connect || !r.pathMap[c.Request.URL.Path] {
		return nil
	}
	return r
}

func (r *FileClient) Disabled() bool { return r == nil || r.config.Disabled }

func (r *FileClient) Do(c *Context) {
	r.log.Debug("Local CDN", zap.Stringer("url", c.Request.URL))
	req := c.Request

	// TODO another way to avoid fs.go redirect?
	if req.URL.Path == "/index.html" {
		req.URL.Path = "/"
	}
	if req.Body != nil {
		req.Body = ioutil.NopCloser(req.Body)
	}
	res, err := r.roundTripper.RoundTrip(req)
	if err != nil {
		r.log.Error("Failed to serve local CDN file", zap.Error(err), zap.Stringer("url", c.Request.URL))
		c.Writer.Write([]byte("HTTP/1.1 502 LocalCDN\r\n\r\n"))
		return
	}
	err = res.Write(c.Writer)
	if err != nil {
		r.log.Error("Failed to write local CDN file to conn", zap.Error(err), zap.Stringer("url", c.Request.URL))
	}
}
