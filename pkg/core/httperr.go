package hybridcore

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"strconv"

	"github.com/empirefox/hybrid/pkg/bufpool"
)

type HttpErr struct {
	Code       int    `json:"-"`
	ClientType string `json:",omitempty"`
	ClientName string `json:",omitempty"`
	TargetHost string `json:",omitempty"`
	Info       string `json:",omitempty"`
}

func (he *HttpErr) Write(w io.Writer) error {
	res, err := he.Response()
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return res.Write(w)
}

func (he *HttpErr) WriteResponse(w http.ResponseWriter) error {
	w.WriteHeader(he.Code)

	res, err := he.Response()
	if err != nil {
		return err
	}
	defer res.Body.Close()

	for k, v := range res.Header {
		w.Header().Set(k, v[0])
	}
	w.Header().Set("Content-Length", strconv.FormatInt(res.ContentLength, 10))
	body := res.Body.(*bufferBody).Bytes()
	_, err = w.Write(body)
	return err
}

func (he *HttpErr) Response() (*http.Response, error) {
	body := newBufferBody(hybridbufpool.Default1K)
	err := json.NewEncoder(body).Encode(he)
	if err != nil {
		body.Close()
		return nil, err
	}

	resp := &http.Response{
		Status:     "Hybrid Error",
		StatusCode: he.Code,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:          body,
		ContentLength: int64(body.Len()),
	}
	return resp, nil
}

type bufferBody struct {
	*bytes.Buffer
	buf  []byte
	pool httputil.BufferPool
}

func newBufferBody(pool httputil.BufferPool) *bufferBody {
	buf := pool.Get()
	return &bufferBody{
		Buffer: bytes.NewBuffer(buf),
		buf:    buf,
		pool:   pool,
	}
}

func (b *bufferBody) Close() error {
	b.pool.Put(b.buf)
	return nil
}
