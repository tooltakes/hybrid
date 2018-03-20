package hybrid

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

func checkRequestEnd(w *io.PipeWriter, c io.Reader, h2FrameSize int) {
	req, err := http.ReadRequest(bufio.NewReaderSize(io.TeeReader(c, w), h2FrameSize))
	if err != nil {
		w.CloseWithError(err)
		return
	}
	defer req.Body.Close()
	_, err = io.Copy(ioutil.Discard, req.Body)
	w.CloseWithError(err)
}

// from go request.go
// parseRequestLine parses "GET /foo HTTP/1.1" into its three parts.
func parseRequestLine(requestLine string) (method, requestURI, proto string, ok bool) {
	s1 := strings.Index(requestLine, " ")
	s2 := strings.Index(requestLine[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return requestLine[:s1], requestLine[s1+1 : s2], requestLine[s2+1:], true
}

// convert from net/textproto/reader.go:Reader.upcomingHeaderNewlines
func peekRequestLine(r *bufio.Reader) ([]byte, error) {
	r.Peek(1) // force a buffer load if empty
	s := r.Buffered()
	if s == 0 {
		return nil, errors.New("No request in connection")
	}
	peek, _ := r.Peek(s)
	line, err := bytes.NewBuffer(peek).ReadBytes('\n')
	if err != nil {
		return nil, errors.New("Bad request in connection")
	}
	return line, nil
}

// from gorilla
func hostPortNoPort(u *url.URL) (hostPort, hostNoPort string) {
	hostPort = u.Host
	hostNoPort = u.Host
	if i := strings.LastIndex(u.Host, ":"); i > strings.LastIndex(u.Host, "]") {
		hostNoPort = hostNoPort[:i]
	} else {
		if u.Scheme == "wss" || u.Scheme == "https" {
			hostPort += ":443"
		} else {
			hostPort += ":80"
		}
	}
	return hostPort, hostNoPort
}

// authorityAddrFull returns a given authority (a host/IP, or host:port / ip:port)
// and returns a host:port. The port 443 is added if needed.
// imported from http2/transport.go
func authorityAddrFull(scheme string, authority string) (hostport, hostnoport, port string) {
	host, port, err := net.SplitHostPort(authority)
	if err != nil { // authority didn't have a port
		port = "443"
		if scheme == "ws" || scheme == "http" {
			port = "80"
		}
		host = authority
	}
	if a, err := idna.ToASCII(host); err == nil {
		host = a
	}
	// IPv6 address literal, without a port:
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host + ":" + port, host, port
	}
	return net.JoinHostPort(host, port), host, port
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}
