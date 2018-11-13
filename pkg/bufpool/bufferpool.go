package bufpool

import (
	"sync"
)

const (
	DefaultSize = 32 << 10
)

type Pool struct {
	pool sync.Pool
}

func New() *Pool {
	return NewSize(0)
}

func NewSize(size int) *Pool {
	if size == 0 {
		size = DefaultSize
	}
	return &Pool{
		pool: sync.Pool{
			New: func() interface{} { return make([]byte, size) },
		},
	}
}

func NewSizeModify(size int, modify func([]byte)) *Pool {
	return &Pool{
		pool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, size)
				modify(b)
				return b
			},
		},
	}
}

func (p *Pool) Get() []byte  { return p.pool.Get().([]byte) }
func (p *Pool) Put(b []byte) { p.pool.Put(b) }

var (
	Default   = New()
	Default1K = NewSize(1024)
)
