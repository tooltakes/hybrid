package main

import (
	"encoding/hex"
	"net"
	"net/url"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/kcp"
	"github.com/xtaci/kcp-go"
)

const (
	key = "9BBAAD48A3769FA8ED10A0B1DD860B9D3A0C109909B1750D"
)

func main() {
	log, err := zap.NewProduction()
	defer log.Sync()
	if err != nil {
		panic(err)
	}

	blocked, err := url.Parse("kcp://127.0.0.1:19995")
	if err != nil {
		log.Error("url.Parse", zap.Error(err))
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
	}

	aeskey, err := hex.DecodeString(strings.ToLower(key))
	if err != nil {
		log.Error("DecodeString", zap.Error(err))
	}
	block, err := kcp.NewAESBlockCrypt(aeskey)
	if err != nil {
		log.Error("NewAESBlockCrypt", zap.Error(err))
	}

	client1 := hybrid.NewClient(hybrid.ClientConfig{
		Log: log,
		Dialer: &hybridkcp.KCPDialer{
			Block:       block,
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

	l, err := net.Listen("tcp", ":7775")
	if err != nil {
		log.Error("Listen", zap.Error(err))
	}

	err = hybrid.SimpleListenAndServe(l, h.Proxy)
	if err != nil {
		log.Error("SimpleListenAndServe", zap.Error(err))
	}
}
