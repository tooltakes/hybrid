package hybrid

import (
	"io"
	"net/http"
	"net/http/httputil"
)

type ResponseWriter struct {
	// code is the HTTP response code set by WriteHeader.
	//
	// Note that if a Handler never calls WriteHeader or Write,
	// this might end up being 0, rather than the implicit
	// http.StatusOK. To get the implicit value, use the Result
	// method.
	code int

	// header contains the headers explicitly set by the Handler.
	//
	// To get the implicit headers set by the server (such as
	// automatic Content-Type), use the Result method.
	header http.Header

	// writer is the buffer to which the Handler's Write calls are sent.
	// If nil, the Writes are silently discarded.
	writer io.Writer

	wroteHeader bool
}

// NewResponseWriter returns an initialized ResponseWriter.
func NewResponseWriter(w io.Writer) *ResponseWriter {
	return &ResponseWriter{
		header: make(http.Header),
		writer: w,
		code:   200,
	}
}

// Header returns the response headers.
func (rw *ResponseWriter) Header() http.Header {
	return rw.header
}

// writeHeader writes a header if it was not written yet and
// detects Content-Type if needed.
//
// bytes or str are the beginning of the response body.
// We pass both to avoid unnecessarily generate garbage
// in rw.WriteString which was created for performance reasons.
// Non-nil bytes win.
func (rw *ResponseWriter) writeHeader(b []byte) {
	if rw.wroteHeader {
		return
	}

	m := rw.Header()

	_, hasType := m["Content-Type"]
	hasTE := m.Get("Transfer-Encoding") != ""
	if !hasType && !hasTE {
		m.Set("Content-Type", http.DetectContentType(b))
	}

	rw.WriteHeader(200)
}

// Write always succeeds and writes to rw.writer, if not nil.
func (rw *ResponseWriter) Write(buf []byte) (int, error) {
	rw.writeHeader(buf)
	if rw.writer != nil {
		rw.writer.Write(buf)
	}
	return len(buf), nil
}

// WriteHeader sets rw.code. After it is called, changing rw.Header
// will not affect rw.header.
func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.code = code
	rw.wroteHeader = true
	head, _ := httputil.DumpResponse(&http.Response{
		StatusCode: code,
		Status:     http.StatusText(code),
		Header:     rw.header,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}, false)
	rw.writer.Write(head)
}
