package hybrid

import (
	"io"
	"net"
	"net/http"
	"time"
)

type TimeoutReader struct {
	Conn         net.Conn
	Timeout      time.Duration
	TimeoutSleep time.Duration
}

func (r *TimeoutReader) Read(buf []byte) (n int, err error) {
	r.Conn.SetReadDeadline(time.Now().Add(r.Timeout))
	n, err = r.Conn.Read(buf)
	return
}

func (r *TimeoutReader) WriteTo(dst io.Writer) (written int64, err error) {
	src := r.Conn
	sleep := r.TimeoutSleep
	buf := make([]byte, 32<<10)
	var flush func()
	if flusher, ok := dst.(http.Flusher); ok {
		flush = flusher.Flush
	}

	// fast read 5 times, then slow read, then repeat after got bytes
	i := 0
	times := 5
	timeout := 200 * time.Millisecond
	for {
		src.SetReadDeadline(time.Now().Add(timeout))
		nr, er := src.Read(buf)
		if nr > 0 {
			i = 0
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				if flush != nil {
					flush()
				}
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
				i++
				if i > times && nr == 0 {
					timeout = r.Timeout
					time.Sleep(sleep)
				}
				err = nil
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
