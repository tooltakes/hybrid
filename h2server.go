package hybrid

import (
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type H2Server struct {
	Log *zap.Logger

	ReverseProxy *httputil.ReverseProxy

	LocalHandler http.Handler
}

func (s *H2Server) Serve(listener net.Listener) error {
	s1 := &http.Server{Handler: s}
	s2 := &http2.Server{}
	http2.ConfigureServer(s1, s2)

	if s.ReverseProxy.FlushInterval == 0 {
		s.ReverseProxy.FlushInterval = 250 * time.Millisecond
	}

	return SimpleListenAndServe(listener, func(c net.Conn) {
		s2.ServeConn(c, &http2.ServeConnOpts{BaseConfig: s1})
		s.Log.Debug("ServeConn end")
	})
}

func (s *H2Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host[1:]

	switch r.Host[0] {
	case HostHttpPrefix:
		r.URL.Host = strings.TrimSuffix(host, ":80")
		r.URL.Scheme = "http"
	case HostHttpsPrefix:
		r.URL.Host = strings.TrimSuffix(host, ":443")
		r.URL.Scheme = "https"
	default:
		switch r.Host {
		case HostLocalServer:
			if s.LocalHandler != nil {
				s.LocalHandler.ServeHTTP(w, r)
			} else {
				w.WriteHeader(http.StatusNotImplemented)
			}
		case HostPing:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	}

	isConnect := r.Method == "CONNECT"
	r.Proto = "HTTP/1.1"
	r.ProtoMajor = 1
	r.ProtoMinor = 1
	r.Host = r.URL.Host

	if !isConnect {
		s.ReverseProxy.ServeHTTP(w, r)
		return
	}

	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.Log.Error("recover", zap.Any("err", err))
		}
	}()

	remote, err := net.DialTimeout("tcp", host, time.Second*30)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	done := make(chan struct{})
	defer close(done)
	if cn, ok := w.(http.CloseNotifier); ok {
		notifyChan := cn.CloseNotify()
		go func() {
			select {
			case <-notifyChan:
			case <-done:
			}
			remote.Close()
		}()
	}

	// for copy
	// if happens at the start of req, will wait at most 0.5s
	// if happens at the end of req, will wait at most 1s
	tr := &TimeoutReader{
		Conn:         remote,
		Timeout:      time.Second / 2,
		TimeoutSleep: time.Second / 2,
	}
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
	go io.Copy(remote, r.Body)
	io.Copy(w, tr)
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
