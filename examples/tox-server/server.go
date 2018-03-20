package main

import (
	"bytes"
	"net/http"
	"net/http/httputil"
	"time"

	"go.uber.org/zap"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/toxenums"
	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridtox"
)

// tox-account
//{
//  "Address": "4F5B696C9CACE807E82C3F2A0B425839DF444C979EFE768EF659F605E6A2D84C03B189A919E8",
//  "Secret": "6D79B8C1AD81EA75757736211EDF8EFAEF2B70FD738A155D6B1A09FF7EDD4E40",
//  "Pubkey": "4F5B696C9CACE807E82C3F2A0B425839DF444C979EFE768EF659F605E6A2D84C",
//  "Nospam": 61966761
//}

func main() {
	log, err := zap.NewProduction()
	defer log.Sync()
	if err != nil {
		panic(err)
	}

	t, err := tox.NewTox(&tox.ToxOptions{
		Savedata_type:           toxenums.TOX_SAVEDATA_TYPE_SECRET_KEY,
		Savedata_data:           tox.MustDecodeSecret("6D79B8C1AD81EA75757736211EDF8EFAEF2B70FD738A155D6B1A09FF7EDD4E40")[:],
		Tcp_port:                0,
		NospamIfSecretType:      61966761,
		ProxyToNoneIfErr:        true,
		AutoTcpPortIfErr:        true,
		DisableTcpPortIfAutoErr: true,
		PingUnit:                time.Second,
	})
	if err != nil {
		log.Error("NewTox", zap.Error(err))
	}

	tt := hybridtox.NewToxTCP(hybridtox.ToxTCPConfig{
		Log:          log,
		Tox:          t,
		DialSecond:   0,
		Supers:       nil,
		Servers:      nil,
		RequestToken: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte) []byte { return []byte("i'm a invalid client") },
		ValidateRequest: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte, message []byte) bool {
			ok := bytes.Equal(message, []byte("i'm a valid client"))
			log.Info("ValidateRequest", zap.Bool("ok", ok))
			return ok
		},

		// every friend may fire multi times
		OnFriendAddErr: func(address *[tox.ADDRESS_SIZE]byte, err error) {
			log.Info("OnFriendAddErr", zap.Error(err))
		},

		// controll tox only
		OnSupperAction: func(friendNumber uint32, action []byte) {
			log.Info("OnSupperAction", zap.ByteString("action", action))
		},
	})

	result := t.BootstrapNodes_l(hybridtox.ToxNodes)
	if result.Error() != nil {
		log.Error("BootstrapNodes_l", zap.Error(result.Error()))
		return
	}

	go t.Run()

	s := &hybrid.Server{
		Log:             log,
		ListenerCreator: tt,
		TLSConfig:       nil,
		ReverseProxy: &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				_, err := httputil.DumpRequest(req, false)
				if err != nil {
					log.Error("DumpRequest", zap.Error(err))
					return
				}
			},
			FlushInterval: time.Second,
		},
	}

	err = s.Serve()
	if err != nil {
		log.Error("Serve", zap.Error(err))
		return
	}
}
