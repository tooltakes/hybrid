package zipfs

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/denormal/go-gitignore"

	"gopkg.in/h2non/filetype.v1"
)

const (
	IgnoreFile      = ".zignore"
	MetaInfoDir     = "META-INF"
	ContentInfoFile = "ContentInfo.toml"
)

type FileInfo struct {
	Type string `toml:",omitempty"`
	Raw  bool   `toml:",omitempty"`
}

type ContentInfo struct {
	Files map[string]*FileInfo
}

func GzipThenZip(root, dstFile string) error {
	dst, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	return GzipThenZipWriter(root, dst)
}

func GzipThenZipWriter(root string, dst io.Writer) error {
	z, err := newGzipThenZip(root)
	if err != nil {
		return err
	}
	return z.generate(dst)
}

type gzipThenZip struct {
	ignore  gitignore.GitIgnore
	root    string
	rootLen int // make fsPath relative
	info    *ContentInfo
	now     time.Time
}

func newGzipThenZip(root string) (*gzipThenZip, error) {
	root = filepath.Clean(root)

	ignore, err := gitignore.NewRepositoryWithFile(root, IgnoreFile)
	if err != nil {
		return nil, err
	}

	rootLen := len(root)
	if root[rootLen-1] != filepath.Separator {
		rootLen++
	}

	return &gzipThenZip{
		ignore:  ignore,
		root:    root,
		rootLen: rootLen,
		now:     time.Now(),
	}, nil
}

func (z *gzipThenZip) generate(dst io.Writer) error {
	zw := zip.NewWriter(dst)
	defer zw.Close() // maybe twice, but no matter.

	// TODO remove from struct
	z.info = &ContentInfo{
		Files: make(map[string]*FileInfo),
	}

	err := filepath.Walk(z.root, func(fsPath string, osfi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fsPath == z.root {
			return nil
		}
		return z.proccessFile(zw, fsPath, osfi)
	})
	if err != nil {
		return err
	}

	// generate META-INF/ContentInfo.toml
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:     fmt.Sprintf("%s/%s", MetaInfoDir, ContentInfoFile),
		Method:   zip.Store,
		Modified: z.now,
	})
	if err != nil {
		return err
	}

	err = toml.NewEncoder(w).Encode(z.info)
	if err != nil {
		return err
	}

	return zw.Close()
}

func (z *gzipThenZip) proccessFile(zw *zip.Writer, fsPath string, osfi os.FileInfo) error {
	imatch := z.ignore.Relative(fsPath[z.rootLen:], false)
	if imatch != nil && imatch.Ignore() {
		if osfi.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}

	if !osfi.Mode().IsRegular() {
		return nil
	}

	entry := filepath.ToSlash(fsPath[z.rootLen:])
	if strings.HasPrefix(entry, MetaInfoDir) {
		return nil
	}

	header, err := zip.FileInfoHeader(osfi)
	if err != nil {
		return err
	}
	header.Name = entry

	fsFile, err := os.Open(fsPath)
	if err != nil {
		return err
	}
	defer fsFile.Close()

	r := bufio.NewReader(fsFile)
	head, err := r.Peek(1024)
	if err != nil && err != io.EOF {
		return err
	}

	// detect mime
	typ, err := filetype.Match(head)
	if err == filetype.ErrEmptyBuffer {
		// mime.TypeByExtension at runtime, do not do more here
		z.info.Files[entry] = &FileInfo{Raw: true}
		_, err = zw.CreateHeader(header)
		return err
	}
	if err != nil {
		return err
	}

	// raw when too small
	if len(head) < 1024 {
		z.info.Files[entry] = &FileInfo{Type: typ.MIME.Value, Raw: true}
		return z.writeFileRaw(zw, header, r)
	}

	ext := filepath.Ext(entry)
	ctype := mime.TypeByExtension(ext)
	if ctype == "" {
		ctype = typ.MIME.Value
		if ctype == "" {
			ctype = http.DetectContentType(head)
		}
		z.info.Files[entry] = &FileInfo{Type: ctype}
	}
	if typ.MIME.Value != "" {
		ext = typ.Extension
	}

	switch ext {
	case "psd",
		"doc",
		"docx",
		"xls",
		"xlsx",
		"ppt",
		"pptx":
		return z.writeFile(zw, header, r)
	}

	gzipWhenContains := []string{
		"text",
		"javascript",
		"html",
		"css",
		"json",
		"xml",
	}
	for _, t := range gzipWhenContains {
		if strings.Contains(ctype, t) {
			return z.writeFile(zw, header, r)
		}
	}

	z.info.Files[entry] = &FileInfo{Type: ctype, Raw: true}
	return z.writeFileRaw(zw, header, r)
}

func (z *gzipThenZip) writeFileRaw(zw *zip.Writer, header *zip.FileHeader, r io.Reader) error {
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, r)
	return err
}

func (z *gzipThenZip) writeFile(zw *zip.Writer, header *zip.FileHeader, r io.Reader) error {
	header.Name += ".gz"
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	gw := gzip.NewWriter(w)

	_, err = io.Copy(gw, r)
	if err != nil {
		return err
	}

	return gw.Close()
}
