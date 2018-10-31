package hybrid

import (
	"io"
	"net"
	"net/http"
	"time"
)

type TimeoutWriterTo struct {
	src SetReadDeadlineReader
}

func NewTimeoutWriterTo(src SetReadDeadlineReader) *TimeoutWriterTo {
	return &TimeoutWriterTo{src}
}

// WriteTo writes all Conn data to dst using buf.
// Every time the Read after timeout, waits for next byte without timeout.
func (r *TimeoutWriterTo) WriteTo(dst io.Writer, buf []byte, timeout time.Duration) (written int64, err error) {
	src := r.src

	if buf == nil {
		buf = make([]byte, 32<<10)
	}

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
			nr, er = src.Read(buf[:1])
		} else {
			if err := src.SetReadDeadline(time.Now().Add(timeout)); err != nil {
				// no timeout, so fallback to io.Copy
				// TODO check Flusher?
				return io.CopyBuffer(dst, src, buf)
			}
			nr, er = src.Read(buf)
		}
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				needFlush = true
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
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

type SetReadDeadlineReader interface {
	Read(b []byte) (n int, err error)
	SetReadDeadline(t time.Time) error
}
