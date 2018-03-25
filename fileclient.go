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
	Dev      bool
	Disabled bool
	DirPath  string
	Redirect map[string]string
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

	if config.Dev {
		dir := filepath.Base(config.DirPath)
		for p, _ := range pathMap {
			config.Log.Debug("PathMap", zap.String("dir", dir), zap.String("file", p))
		}
	}

	return &FileClient{
		log:          config.Log,
		config:       config,
		roundTripper: http.NewFileTransport(http.Dir(config.DirPath)),
		pathMap:      pathMap,
	}, nil
}

func (r *FileClient) Route(c *Context) Proxy {
	if r != nil && !c.Connect && r.pathMap[c.Request.URL.Path] {
		if r.config.Dev {
			r.log.Debug("Local Y", zap.String("host", c.Request.URL.Host), zap.String("path", c.Request.URL.Path))
		}
		return r
	}
	if r != nil && r.config.Redirect != nil {
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
	res, err := r.roundTripper.RoundTrip(req)
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
