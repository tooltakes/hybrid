package main

import (
	"encoding/hex"
	"net/http"
	"net/http/httputil"
	"time"

	"go.uber.org/zap"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/toxenums"
	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridauth"
	"github.com/empirefox/hybrid/hybridtox"
)

func main() {
	log, err := zap.NewDevelopment()
	defer log.Sync()
	if err != nil {
		panic(err)
	}

	config, err := LoadConfig()
	if err != nil {
		log.Fatal("Parse config file", zap.Error(err))
	}

	toxSecret, err := tox.DecodeSecret(config.SecretHex)
	if err != nil {
		log.Fatal("Parse toxSecret", zap.Error(err))
	}

	verifyKey, err := tox.DecodePubkey(config.VerifyKeyHex)
	if err != nil {
		log.Fatal("Parse verifyKey", zap.Error(err))
	}
	ver := Verifier(*verifyKey)
	verifier := hybridauth.Verifier{&ver, &ver}

	t, err := tox.NewTox(&tox.ToxOptions{
		Savedata_type:           toxenums.TOX_SAVEDATA_TYPE_SECRET_KEY,
		Savedata_data:           toxSecret[:],
		Tcp_port:                0,
		NospamIfSecretType:      config.Nospam,
		ProxyToNoneIfErr:        true,
		AutoTcpPortIfErr:        true,
		DisableTcpPortIfAutoErr: true,
		PingUnit:                time.Second,
	})
	if err != nil {
		log.Fatal("NewTox", zap.Error(err))
	}
	defer t.Kill()

	tt := hybridtox.NewToxTCP(hybridtox.ToxTCPConfig{
		Log:          log,
		Tox:          t,
		Supers:       nil,
		Servers:      nil,
		RequestToken: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte) []byte { return []byte("i'm a invalid client") },
		ValidateRequest: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte, message []byte) bool {
			var pk [64]byte
			hex.Encode(pk[:], pubkey[:])
			_, err := verifier.Verify(pk[:], message)
			if err != nil {
				if err != nil {
					log.Info("VerifyClient", zap.ByteString("pk", pk[:]), zap.Error(err))
				}
			}
			return err == nil
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
		log.Fatal("BootstrapNodes_l", zap.Error(result.Error()))
	}

	go t.Run()
	defer t.Stop()

	s := &hybrid.H2Server{
		Log: log,
		ReverseProxy: &httputil.ReverseProxy{
			Director:      func(req *http.Request) {},
			FlushInterval: time.Second,
		},
		LocalHandler: nil,
	}

	err = s.Serve(t)
	if err != nil {
		log.Fatal("Serve", zap.Error(err))
	}
	_ = tt
}
