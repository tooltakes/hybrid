package main

import (
	"encoding/hex"
	"net"
	"net/url"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/empirefox/cement/clog"
	"github.com/empirefox/hybrid"
)

var (
	serverPubHex    = []byte(strings.ToLower("9AA0FF6C243F90947035FA4AA45353A705B4F9681299AC61A295E3C32911EB63"))
	clientScalarHex = []byte(strings.ToLower("4C479CBC289E1085D90E5B951DCFBCA27940C3B9182AC79DDDAC1DE36CA2A967"))
)

func main() {
	cl, err := clog.NewLogger(clog.Config{
		Dev:   true,
		Level: "debug",
	})
	if err != nil {
		panic(err)
	}

	log := cl.Module("hybrid")

	blocked, err := url.Parse("tcp://127.0.0.1:19999")
	if err != nil {
		log.Error("url.Parse", zap.Error(err))
		return
	}

	router1, err := hybrid.NewAdpRouter(&hybrid.AdpRouterConfig{
		Log:       log,
		Disabled:  false,
		Exist:     os.Getenv("EXIST"),
		Blocked:   blocked,
		Unblocked: hybrid.DIRECT,
		B64Rules:  [][]byte{[]byte(rules0)},
		TxtRules:  [][]byte{[]byte(rules1)},
	})
	if err != nil {
		log.Error("NewAdpRouter", zap.Error(err))
		return
	}

	var serverPub, clientScalar [32]byte
	_, err = hex.Decode(serverPub[:], serverPubHex)
	if err != nil {
		log.Error("serverPub", zap.Error(err))
		return
	}
	_, err = hex.Decode(clientScalar[:], clientScalarHex)
	if err != nil {
		log.Error("clientScalar", zap.Error(err))
		return
	}

	cryptoConfig1 := &hybrid.CryptoConnConfig{
		ServerPublic: &serverPub,
		ClientScalar: &clientScalar,
	}
	client1 := hybrid.NewClient(hybrid.ClientConfig{
		Log: log,
		Dialer: &hybrid.TCPDialer{
			Config:      cryptoConfig1,
			ServerNames: nil,
		},
		NoTLS:           true,
		TLSClientConfig: nil,
	})

	rc1 := hybrid.NewRouteClient(router1, client1)

	_, net0, _ := net.ParseCIDR("192.168.64.0/24")
	router0 := &hybrid.NetRouter{
		Config: &hybrid.NetRouterConfig{
			Disabled: false,
			Exist:    "127.0.0.1:8899",
			IPs:      nil,
			Nets: map[*net.IPNet]*url.URL{
				net0: hybrid.EXIST,
			},
			Unmatched: hybrid.NEXT,
		},
	}

	// client can be nil if h2 not needed
	rc0 := hybrid.NewRouteClient(router0, nil)

	h := &hybrid.Hybrid{
		Routers: []hybrid.RouteClient{rc0, rc1},
	}

	l, err := net.Listen("tcp", ":7777")
	if err != nil {
		log.Error("Listen", zap.Error(err))
		return
	}

	err = hybrid.SimpleListenAndServe(l, h.Proxy)
	if err != nil {
		log.Error("SimpleListenAndServe", zap.Error(err))
	}
}
