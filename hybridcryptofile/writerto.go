package hybridcryptofile

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/aead/poly1305"
	"golang.org/x/crypto/chacha20poly1305"
)

type writerTo struct {
	CryptoConfig
	encrypt    bool
	r          io.Reader
	nn         int64 // total read from new
	header     *Header
	aead       cipher.AEAD
	block      int
	bufferPool *sync.Pool
	cryptCh    chan *buffer
	writeCh    chan *buffer
	cryptErrCh chan error
	doneCh     chan error
	cancelCh   chan struct{}
}

func NewEncryptWriterTo(cc CryptoConfig, h *Header, r io.Reader) (io.WriterTo, error) {
	if len(cc.Password) == 0 {
		return nil, ErrEmptyPassword
	}

	if h == nil {
		h = new(Header)
	}
	if h.Version != 0 && h.Version != Version {
		return nil, fmt.Errorf("only support v%d, but got v%d", Version, h.Version)
	}
	if h.Argon2Salt == nil {
		var s [32]byte
		_, err := io.ReadFull(rand.Reader, s[:])
		if err != nil {
			return nil, err
		}
		h.Argon2Salt = &s
	}

	h.Version = Version
	if h.BlockKB == 0 {
		h.BlockKB = BlockKB
	}
	if h.Argon2Time == 0 {
		h.Argon2Time = Argon2Time
	}
	if h.Argon2Memory == 0 {
		h.Argon2Memory = Argon2Memory
	}
	if h.Argon2Threads == 0 {
		h.Argon2Threads = Argon2Threads
	}
	return newWriterTo(&cc, h, r, true)
}

func NewDecryptWriterTo(cc CryptoConfig, r io.Reader) (io.WriterTo, int, error) {
	if len(cc.Password) == 0 {
		return nil, 0, ErrEmptyPassword
	}

	h, n, err := NewHeader(r)
	if err != nil {
		return nil, n, err
	}

	if h.Version != Version {
		return nil, n, fmt.Errorf("only support v%d, but got v%d", Version, h.Version)
	}

	wt, err := newWriterTo(&cc, h, r, false)
	return wt, n, err
}

func newWriterTo(cc *CryptoConfig, h *Header, r io.Reader, encrypt bool) (*writerTo, error) {
	k, err := h.KeyDerive(cc.Password)
	if err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.New(k)
	if err != nil {
		return nil, err
	}

	if cc.BlockQueue == 0 {
		cc.BlockQueue = BlockQueue
	}

	block := int(h.BlockKB) << 10
	if cc.GetBufferPool == nil {
		cc.GetBufferPool = NewBufferPool
	}

	return &writerTo{
		CryptoConfig: *cc,
		encrypt:      encrypt,
		r:            r,
		header:       h,
		aead:         aead,
		block:        block,
		bufferPool:   cc.GetBufferPool(block),
		cryptCh:      make(chan *buffer, cc.BlockQueue),
		writeCh:      make(chan *buffer, cc.BlockQueue),
		cryptErrCh:   make(chan error),
		doneCh:       make(chan error),
		cancelCh:     make(chan struct{}),
	}, nil
}

func (r *writerTo) WriteTo(w io.Writer) (int64, error) {
	if r.encrypt {
		_, err := w.Write(r.header.Bytes()[:])
		if err != nil {
			return 0, err
		}
	} else {
		r.nn = HeaderLen
	}

	go r.writeLoop(w)
	go r.cryptLoop()

	defer close(r.doneCh)
	defer close(r.cryptErrCh)

	var nonce uint64
	for {
		// get buf
		buf := r.bufferPool.Get().(*buffer).Init(r.encrypt)

		// set nonce, we use a len of 12
		for i := 0; i < 4; i++ {
			buf.nonce[i] = 0
		}
		nonce++
		binary.BigEndian.PutUint64(buf.nonce[4:], nonce)

		n, err := io.ReadFull(r.r, buf.src)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				// send last block, then act like EOF
				select {
				case r.cryptCh <- buf.WithRead(n):
				case err = <-r.doneCh:
					close(r.cancelCh)
					return r.nn, err
				}
				err = io.EOF
			} else {
				// do not sent empty block or read error happened
				r.bufferPool.Put(buf)
			}
			if err == io.EOF {
				// EOF is not error here
				// do not close cancelCh, because crypt and write may not be done
				close(r.cryptCh)
				err = nil
			} else {
				close(r.cancelCh)
			}
			derr := <-r.doneCh
			if derr != nil {
				err = derr
			}
			return r.nn, err
		}

		select {
		case r.cryptCh <- buf:
		case err = <-r.doneCh:
			close(r.cancelCh)
			return r.nn, err
		}
	}
}

func (r *writerTo) cryptLoop() {
	// do not close cryptErrCh, because write may not be done
	defer close(r.writeCh)
	for {
		select {
		case buf, ok := <-r.cryptCh:
			if !ok {
				return
			}

			if r.encrypt {
				r.aead.Seal(buf.dst, buf.nonce, buf.plaintext, nil)
			} else {
				_, err := r.aead.Open(buf.dst, buf.nonce, buf.ciphertext, nil)
				if err != nil {
					select {
					case r.cryptErrCh <- err:
					case <-r.cancelCh:
					}
					return
				}
			}
			select {
			case r.writeCh <- buf:
			case <-r.cancelCh:
				return
			}
		case <-r.cancelCh:
			return
		}
	}
}

func (r *writerTo) writeLoop(w io.Writer) {
	for {
		select {
		case buf, ok := <-r.writeCh:
			if !ok {
				r.doneCh <- nil
				return
			}
			n, err := w.Write(buf.result)
			r.bufferPool.Put(buf)
			if r.encrypt {
				r.nn += int64(n - poly1305.TagSize)
			} else {
				r.nn += int64(n + poly1305.TagSize)
			}
			if err != nil {
				r.doneCh <- err
				return
			}
		case err := <-r.cryptErrCh:
			r.doneCh <- err
			return
		case <-r.cancelCh:
			r.doneCh <- nil
			return
		}
	}
}
