package hybridhttp

import (
	"io"
	"net"
	"net/http"
	"time"
)

type FromWriterError struct{ error }

type TimeoutWriterTo struct {
	SetReadDeadlineReadCloser
	buf     []byte
	timeout time.Duration
}

func NewTimeoutWriterTo(src SetReadDeadlineReadCloser, buf []byte, timeout time.Duration) *TimeoutWriterTo {
	return &TimeoutWriterTo{src, buf, timeout}
}

// WriteTo writes all Conn data to dst using buf.
// Every time the Read after timeout, waits for next byte without timeout.
func (r *TimeoutWriterTo) WriteTo(dst io.Writer) (written int64, err error) {
	buf := r.buf
	if buf == nil {
		buf = make([]byte, 32<<10)
	}

	timeout := r.timeout
	if timeout == 0 {
		timeout = 200 * time.Millisecond
	}

	var flush = func() {}
	if flusher, ok := dst.(http.Flusher); ok {
		flush = flusher.Flush
	}

	afterTimeout := false
	needFlush := false
	var nr int
	var er error
	for {
		if afterTimeout {
			afterTimeout = false
			n, e := r.read(buf[:1], 0)
			if e == nil {
				nr, er = r.read(buf[n:], timeout)
				nr += n
			}
		} else {
			nr, er = r.read(buf, timeout)
		}

		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				needFlush = true
				written += int64(nw)
			}
			if ew != nil {
				err = FromWriterError{ew}
				break
			}
			if nr != nw {
				err = FromWriterError{io.ErrShortWrite}
				break
			}
		}
		if er != nil {
			if nerr, ok := er.(net.Error); ok && nerr.Timeout() {
				if needFlush {
					needFlush = false
					flush()
				}
				afterTimeout = true
			} else {
				if er != io.EOF {
					err = er
				}
				break
			}
		}
	}
	return written, err
}

func (r *TimeoutWriterTo) read(buf []byte, timeout time.Duration) (n int, err error) {
	if timeout == 0 {
		r.SetReadDeadline(time.Time{})
	} else {
		r.SetReadDeadline(time.Now().Add(timeout))
	}
	return r.Read(buf)
}

type SetReadDeadlineReadCloser interface {
	Read(b []byte) (n int, err error)
	Close() error
	SetReadDeadline(t time.Time) error
}
