package hybrid

import (
	"fmt"
	"io"
	"net/http"
)

// handlerTransport implements RoundTripper for the 'file' protocol.
type handlerTransport struct {
	handlerFunc http.HandlerFunc
}

// NewFileTransport returns a new RoundTripper, serving the provided
// FileSystem. The returned RoundTripper ignores the URL host in its
// incoming requests, as well as most other properties of the
// request.
//
// The typical use case for NewFileTransport is to register the "file"
// protocol with a Transport, as in:
//
//   t := &http.Transport{}
//   t.RegisterProtocol("file", http.NewHandlerTransport(http.FileServer(root)))
//   c := &http.Client{Transport: t}
//   res, err := c.Get("file:///etc/passwd")
//   ...
func NewHandlerTransport(handlerFunc http.HandlerFunc) http.RoundTripper {
	return handlerTransport{handlerFunc}
}

func (t handlerTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// We start ServeHTTP in a goroutine, which may take a long
	// time if the file is large. The newPopulateResponseWriter
	// call returns a channel which either ServeHTTP or finish()
	// sends our *Response on, once the *Response itself has been
	// populated (even if the body itself is still being
	// written to the res.Body, a pipe)
	rw, resc := newPopulateResponseWriter()
	go func() {
		t.handlerFunc(rw, req)
		rw.finish()
	}()
	return <-resc, nil
}

func newPopulateResponseWriter() (*populateResponse, <-chan *http.Response) {
	pr, pw := io.Pipe()
	rw := &populateResponse{
		ch: make(chan *http.Response),
		pw: pw,
		res: &http.Response{
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			Header:     make(http.Header),
			Close:      true,
			Body:       pr,
		},
	}
	return rw, rw.ch
}

// populateResponse is a ResponseWriter that populates the *Response
// in res, and writes its body to a pipe connected to the response
// body. Once writes begin or finish() is called, the response is sent
// on ch.
type populateResponse struct {
	res          *http.Response
	ch           chan *http.Response
	wroteHeader  bool
	hasContent   bool
	sentResponse bool
	pw           *io.PipeWriter
}

func (pr *populateResponse) finish() {
	if !pr.wroteHeader {
		pr.WriteHeader(500)
	}
	if !pr.sentResponse {
		pr.sendResponse()
	}
	pr.pw.Close()
}

func (pr *populateResponse) sendResponse() {
	if pr.sentResponse {
		return
	}
	pr.sentResponse = true

	if pr.hasContent {
		pr.res.ContentLength = -1
	}
	pr.ch <- pr.res
}

func (pr *populateResponse) Header() http.Header {
	return pr.res.Header
}

func (pr *populateResponse) WriteHeader(code int) {
	if pr.wroteHeader {
		return
	}
	pr.wroteHeader = true

	pr.res.StatusCode = code
	pr.res.Status = fmt.Sprintf("%d %s", code, http.StatusText(code))
}

func (pr *populateResponse) Write(p []byte) (n int, err error) {
	if !pr.wroteHeader {
		pr.WriteHeader(http.StatusOK)
	}
	pr.hasContent = true
	if !pr.sentResponse {
		pr.sendResponse()
	}
	return pr.pw.Write(p)
}
