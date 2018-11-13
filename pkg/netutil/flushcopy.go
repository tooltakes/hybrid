package netutil

import (
	"io"
	"net/http"
	"sync"
	"time"
)

// CopyBufferFlush copied from httputil.ReverseProxy
func CopyBufferFlush(dst io.Writer, src io.Reader, buf []byte, flushInterval time.Duration) (int64, error) {
	if flushInterval != 0 {
		if wf, ok := dst.(writeFlusher); ok {
			mlw := &maxLatencyWriter{
				dst:     wf,
				latency: flushInterval,
				done:    make(chan bool),
			}
			go mlw.flushLoop()
			defer mlw.stop()
			dst = mlw

			return copyBuffer(dst, src, buf)
		}
	}

	return io.CopyBuffer(dst, src, buf)
}

// copyBuffer returns any write errors or non-EOF read errors, and the amount
// of bytes written.
func copyBuffer(dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	if len(buf) == 0 {
		buf = make([]byte, 32*1024)
	}
	var written int64
	for {
		nr, rerr := src.Read(buf)
		//		if rerr != nil && rerr != io.EOF && rerr != context.Canceled {
		//			p.logf("httputil: ReverseProxy read error during body copy: %v", rerr)
		//		}
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if werr != nil {
				return written, werr
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				rerr = nil
			}
			return written, rerr
		}
	}
}

type writeFlusher interface {
	io.Writer
	http.Flusher
}

type maxLatencyWriter struct {
	dst     writeFlusher
	latency time.Duration

	mu   sync.Mutex // protects Write + Flush
	done chan bool
}

func (m *maxLatencyWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dst.Write(p)
}

func (m *maxLatencyWriter) flushLoop() {
	t := time.NewTicker(m.latency)
	defer t.Stop()
	for {
		select {
		case <-m.done:
			//			if onExitFlushLoop != nil {
			//				onExitFlushLoop()
			//			}
			return
		case <-t.C:
			m.mu.Lock()
			m.dst.Flush()
			m.mu.Unlock()
		}
	}
}

func (m *maxLatencyWriter) stop() { m.done <- true }
