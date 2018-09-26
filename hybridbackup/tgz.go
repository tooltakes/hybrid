package hybridbackup

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrDirNotExist = errors.New("dir does not exist")
)

func TgzBuffer(root string, w io.Writer, buf []byte) error {
	// root=~/.hybrid
	// root will be trimed
	root = filepath.Clean(root)

	info, err := os.Stat(root)
	if err != nil {
		return nil
	}

	if !info.IsDir() {
		return ErrDirNotExist
	}

	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)
	defer gw.Close()
	defer tw.Close()

	rootLen := len(root)
	if root[rootLen-1] != filepath.Separator {
		rootLen++
	}

	if buf == nil {
		buf = make([]byte, 32<<10)
	}
	err = filepath.Walk(root, func(fsPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// only accept regular file
		if !info.Mode().IsRegular() {
			return nil
		}

		header := &tar.Header{
			Name:    filepath.ToSlash(fsPath[rootLen:]),
			Mode:    0600, // u=rw,g=---,o=---
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(fsPath)
		if err != nil {
			return err
		}
		defer fsFile.Close()
		_, err = io.CopyBuffer(tw, fsFile, buf)
		return err
	})
	if err != nil {
		return err
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	return gw.Close()
}

func UnTgzBuffer(dst string, r io.Reader, buf []byte) error {
	// dst=~/.hybrid
	// dst will be prefixed
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

	dst = filepath.Clean(dst)
	if buf == nil {
		buf = make([]byte, 32<<10)
	}

	untar := func(fsPath string, header *tar.Header) error {
		fsFile, err := os.Create(fsPath)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		err = fsFile.Chmod(os.FileMode(header.Mode))
		if err != nil {
			return err
		}

		_, err = io.CopyBuffer(fsFile, tr, buf)
		if err != nil {
			return err
		}

		return fsFile.Close()
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fsPath := filepath.Join(dst, filepath.FromSlash(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(fsPath, os.FileMode(header.Mode))

		case tar.TypeReg:
			err = untar(fsPath, header)

		default:
			err = fmt.Errorf("Unable to untar type: %c in file %s", header.Typeflag, fsPath)
		}

		if err != nil {
			return err
		}
	}

	return gr.Close()
}
