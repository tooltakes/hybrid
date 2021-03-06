package backup

import (
	"io"
	"os"

	"github.com/empirefox/hybrid/pkg/cryptofile"
)

func Restore(root, src string, cc cryptofile.CryptoConfig) error {
	srcFile, err := os.Create(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	r, w := io.Pipe()
	defer w.Close()

	wt, _, err := cryptofile.NewDecryptWriterTo(cc, srcFile)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		err := UnTgzBuffer(root, r, nil)
		if err != nil {
			r.Close()
		}
		errCh <- err
	}()

	_, err = wt.WriteTo(w)
	if err != nil {
		w.Close()
		return err
	}

	return <-errCh
}

func Backup(root, dst string, cc cryptofile.CryptoConfig, h *cryptofile.Header) error {
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	r, w := io.Pipe()
	defer r.Close()

	wt, err := cryptofile.NewEncryptWriterTo(cc, h, r)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		err := TgzBuffer(root, w, nil)
		if err != nil {
			w.Close()
		}
		errCh <- err
	}()

	_, err = wt.WriteTo(dstFile)
	if err != nil {
		r.Close()
		return err
	}

	err = <-errCh
	if err != nil {
		return err
	}

	return dstFile.Close()
}
