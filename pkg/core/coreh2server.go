package hybridcore

import (
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/http2"

	"github.com/empirefox/hybrid/pkg/http"
)

const (
	HostHttpPrefix  byte = 'H'
	HostHttpsPrefix byte = 'S'
)

func (core *Core) Serve(listener net.Listener) error {
	s1 := &http.Server{Handler: core}
	s2 := &http2.Server{}
	http2.ConfigureServer(s1, s2)

	return hybridhttp.SimpleListenAndServe(listener, func(c net.Conn) {
		s2.ServeConn(c, &http2.ServeConnOpts{BaseConfig: s1})
		core.Log.Debug("ServeConn end")
	})
}

func (core *Core) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	c, err := NewContextFromHandler(core.ContextConfig, w, r)
	if err != nil {
		he := HttpErr{
			Code:       http.StatusBadRequest,
			ClientType: "Hybrid",
			ClientName: "CTX",
			TargetHost: r.URL.Host,
			Info:       err.Error(),
		}
		he.WriteResponse(w)
		return
	}
	core.Proxy(c)
}
