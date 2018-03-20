package main

import (
	"net"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/toxenums"
	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridtox"
)

// tox-account
//{
//  "Address": "1D7321E77F37290E0785FE1A919C1CB5BA3F422F74F0A34006A02FC2CF100B1B282AE9611D99",
//  "Secret": "9A3DC948A39ABD654BD5514F0586546FEDF28AECC34921C1BFE38F849D23C836",
//  "Pubkey": "1D7321E77F37290E0785FE1A919C1CB5BA3F422F74F0A34006A02FC2CF100B1B",
//  "Nospam": 673900897
//}

var (
	serverAddress = tox.MustDecodeAddress("4F5B696C9CACE807E82C3F2A0B425839DF444C979EFE768EF659F605E6A2D84C03B189A919E8")
)

func main() {
	log, err := zap.NewProduction()
	defer log.Sync()
	if err != nil {
		panic(err)
	}

	blocked, err := url.Parse("tox://4F5B696C9CACE807E82C3F2A0B425839DF444C979EFE768EF659F605E6A2D84C03B189A919E8")
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

	t, err := tox.NewTox(&tox.ToxOptions{
		Savedata_type:           toxenums.TOX_SAVEDATA_TYPE_SECRET_KEY,
		Savedata_data:           tox.MustDecodeSecret("9A3DC948A39ABD654BD5514F0586546FEDF28AECC34921C1BFE38F849D23C836")[:],
		Tcp_port:                33445,
		NospamIfSecretType:      673900897,
		ProxyToNoneIfErr:        true,
		AutoTcpPortIfErr:        true,
		DisableTcpPortIfAutoErr: true,
		PingUnit:                time.Second,
	})
	if err != nil {
		log.Error("NewTox", zap.Error(err))
	}

	tt := hybridtox.NewToxTCP(hybridtox.ToxTCPConfig{
		Log:             log,
		Tox:             t,
		DialSecond:      60,
		Supers:          nil,
		Servers:         []*[tox.ADDRESS_SIZE]byte{serverAddress},
		RequestToken:    func(pubkey *[tox.PUBLIC_KEY_SIZE]byte) []byte { return []byte("i'm a valid client") },
		ValidateRequest: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte, message []byte) bool { return false },

		// every friend may fire multi times
		OnFriendAddErr: func(address *[tox.ADDRESS_SIZE]byte, err error) {
			log.Info("OnFriendAddErr", zap.Error(err))
		},

		// controll tox only
		OnSupperAction: func(friendNumber uint32, action []byte) {
			log.Info("OnSupperAction", zap.ByteString("action", action))
		},
	})

	client1 := hybrid.NewClient(hybrid.ClientConfig{
		Log:             log,
		Dialer:          tt,
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

	result := t.BootstrapNodes_l(hybridtox.ToxNodes)
	if result.Error() != nil {
		log.Error("BootstrapNodes_l", zap.Error(result.Error()))
		return
	}

	go t.Run()

	err = hybrid.SimpleListenAndServe(l, h.Proxy)
	if err != nil {
		log.Error("SimpleListenAndServe", zap.Error(err))
	}
}
