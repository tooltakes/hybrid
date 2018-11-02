package hybridutils

import (
	"io"

	"github.com/empirefox/hybrid/pkg/bufpool"
)

func Copy(dst io.Writer, src io.Reader) (int64, error) {
	buf := hybridbufpool.Default.Get()
	defer hybridbufpool.Default.Put(buf)
	return io.CopyBuffer(dst, src, buf)
}
