package zipfs

import (
	"archive/zip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/zipfs"
)

type GzipHttpfs struct {
	metaInfo       *ContentInfo
	gfs            gzipFileSystem
	httpFileServer http.Handler
	roundTripper   http.RoundTripper
}

func New(gzipThenZipFile string) (*GzipHttpfs, io.Closer, error) {
	rc, err := zip.OpenReader(gzipThenZipFile)
	if err != nil {
		return nil, nil, err
	}

	hfs, err := NewFileSystem(zipfs.New(rc, gzipThenZipFile))
	if err != nil {
		rc.Close()
		return nil, nil, err
	}

	return hfs, rc, nil
}

func NewReader(r io.ReaderAt, size int64, name string) (*GzipHttpfs, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	// No call to Close, so we may do this
	// TODO test
	return NewFileSystem(zipfs.New(&zip.ReadCloser{Reader: *zr}, name))
}

func NewFileSystem(fs vfs.FileSystem) (*GzipHttpfs, error) {
	rcs, err := fs.Open(fmt.Sprintf("/%s/%s", MetaInfoDir, ContentInfoFile))
	if err != nil {
		return nil, err
	}
	defer rcs.Close()

	metaInfo := new(ContentInfo)
	_, err = toml.DecodeReader(rcs, metaInfo)
	if err != nil {
		return nil, err
	}

	gfs := gzipFileSystem{FileSystem: fs, ContentInfo: metaInfo}
	hfs := httpfs.New(gfs)
	httpFileServer := http.FileServer(hfs)
	return &GzipHttpfs{
		metaInfo:       metaInfo,
		gfs:            gfs,
		httpFileServer: httpFileServer,
		roundTripper:   http.NewFileTransport(hfs),
	}, nil
}

func (hfs *GzipHttpfs) CanRequest(path string) bool {
	_, err := hfs.gfs.Stat(path)
	return err == nil
}

func (hfs *GzipHttpfs) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	info, ok, ctype := hfs.contentInfoType(req.URL.Path)
	w.Header().Set("Content-Type", ctype)
	if !(ok && info.Raw) {
		w = gzipResponseWriter{w}
	}
	hfs.httpFileServer.ServeHTTP(w, req)
}

func (hfs *GzipHttpfs) RoundTrip(req *http.Request) (res *http.Response, err error) {
	res, err = hfs.roundTripper.RoundTrip(req)
	if err != nil {
		return
	}

	info, ok, ctype := hfs.contentInfoType(req.URL.Path)
	res.Header.Set("Content-Type", ctype)
	if !(ok && info.Raw) {
		switch res.StatusCode {
		case http.StatusOK, http.StatusPartialContent:
			res.Header.Set("Content-Encoding", "gzip")
		}
	}
	return
}

func (hfs *GzipHttpfs) contentInfoType(path string) (info *FileInfo, ok bool, ctype string) {
	info, ok = hfs.metaInfo.Files[path[1:]]
	if ok {
		ctype = info.Type
	}
	if ctype == "" {
		ctype = mime.TypeByExtension(filepath.Ext(path))
	}
	return
}

type gzipFileInfo struct {
	os.FileInfo
}

func (fi gzipFileInfo) Name() string {
	name := fi.FileInfo.Name()
	return name[:len(name)-3]
}

type gzipFileSystem struct {
	*ContentInfo
	vfs.FileSystem
}

func (fs gzipFileSystem) gz(name string) (string, bool) {
	info, ok := fs.Files[name[1:]]
	if ok && info.Raw {
		return name, true
	}
	return name + ".gz", false
}

func (fs gzipFileSystem) Open(name string) (vfs.ReadSeekCloser, error) {
	name, _ = fs.gz(name)
	return fs.FileSystem.Open(name)
}

func (fs gzipFileSystem) Lstat(path string) (fi os.FileInfo, err error) {
	p, raw := fs.gz(path)
	fi, err = fs.FileSystem.Lstat(p)
	if os.IsNotExist(err) {
		raw = true
		fi, err = fs.FileSystem.Stat(path)
	}
	if err != nil {
		return
	}
	if !raw {
		fi = gzipFileInfo{fi}
	}
	return
}

func (fs gzipFileSystem) Stat(path string) (fi os.FileInfo, err error) {
	p, raw := fs.gz(path)
	fi, err = fs.FileSystem.Stat(p)
	if os.IsNotExist(err) {
		raw = true
		fi, err = fs.FileSystem.Stat(path)
	}
	if err != nil {
		return
	}
	if !raw {
		fi = gzipFileInfo{fi}
	}
	return
}

func (fs gzipFileSystem) ReadDir(path string) (fis []os.FileInfo, err error) {
	prefix := path[1:] + "/"
	fis, err = fs.FileSystem.ReadDir(path)
	if err != nil {
		return
	}
	for i, fi := range fis {
		name := fi.Name()
		if strings.HasSuffix(name, ".gz") {
			info, ok := fs.Files[prefix+name]
			if !(ok && info.Raw) {
				fis[i] = gzipFileInfo{fi}
			}
		}
	}
	return
}

type gzipResponseWriter struct {
	http.ResponseWriter
}

func (w gzipResponseWriter) WriteHeader(code int) {
	switch code {
	case http.StatusOK, http.StatusPartialContent:
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.ResponseWriter.WriteHeader(code)
}
