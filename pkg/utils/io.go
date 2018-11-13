package utils

import (
	"io"

	"github.com/empirefox/hybrid/pkg/bufpool"
)

func Copy(dst io.Writer, src io.Reader) (int64, error) {
	buf := bufpool.Default.Get()
	defer bufpool.Default.Put(buf)
	return io.CopyBuffer(dst, src, buf)
}
