package hybrid

import (
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

func (h *Hybrid) Serve(listener net.Listener) error {
	s1 := &http.Server{Handler: h}
	s2 := &http2.Server{}
	http2.ConfigureServer(s1, s2)

	return SimpleListenAndServe(listener, func(c net.Conn) {
		s2.ServeConn(c, &http2.ServeConnOpts{BaseConfig: s1})
		h.Log.Debug("ServeConn end")
	})
}

func (h *Hybrid) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host[1:]

	switch r.Host[0] {
	case HostHttpPrefix:
		r.URL.Host = strings.TrimSuffix(host, ":80")
		r.URL.Scheme = "http"
	case HostHttpsPrefix:
		r.URL.Host = strings.TrimSuffix(host, ":443")
		r.URL.Scheme = "https"
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	r.ProtoMajor = 1
	r.ProtoMinor = 1
	r.Host = r.URL.Host

	h.Proxy(NewContextFromHandler(h.ContextConfig, w, r))
}

func SimpleListenAndServe(listener net.Listener, serveConn func(c net.Conn)) error {
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		c, e := listener.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0
		go serveConn(c)
	}
}
