package hybrid

import (
	"io"
	"sync"
)

type BufferPool struct {
	pool sync.Pool
}

func NewBufferPool() *BufferPool {
	return NewBufferPoolSize(0)
}

func NewBufferPoolSize(size int) *BufferPool {
	if size == 0 {
		size = 32 << 10
	}
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} { return make([]byte, size) },
		},
	}
}

func (p *BufferPool) Get() []byte  { return p.pool.Get().([]byte) }
func (p *BufferPool) Put(b []byte) { p.pool.Put(b) }

var (
	DefaultBufferPool   = NewBufferPool()
	DefaultBufferPool1K = NewBufferPoolSize(1024)
)

func Copy(dst io.Writer, src io.Reader) (int64, error) {
	buf := DefaultBufferPool.Get()
	defer DefaultBufferPool.Put(buf)
	return io.CopyBuffer(dst, src, buf)
}
