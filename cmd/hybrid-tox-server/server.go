package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os/user"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/toxenums"
	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridauth"
	"github.com/empirefox/hybrid/hybridtox"
)

func main() {
	log, err := zap.NewProduction()
	defer log.Sync()
	if err != nil {
		panic(err)
	}

	cu, err := user.Current()
	if err != nil {
		log.Fatal("user.Current", zap.Error(err))
	}

	configFile := flag.String("config", filepath.Join(cu.HomeDir, "hybrid.json"), "config file path")
	flag.Parse()

	configContent, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatal("Read config file", zap.Error(err))
	}

	var config Config
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		log.Fatal("Parse config file", zap.Error(err))
	}

	toxSecret, err := tox.DecodeSecret(config.ToxSecretHex)
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
		NospamIfSecretType:      61966761,
		ProxyToNoneIfErr:        true,
		AutoTcpPortIfErr:        true,
		DisableTcpPortIfAutoErr: true,
		PingUnit:                time.Second,
	})
	if err != nil {
		log.Fatal("NewTox", zap.Error(err))
	}

	tt := hybridtox.NewToxTCP(hybridtox.ToxTCPConfig{
		Log:          log,
		Tox:          t,
		DialSecond:   0,
		Supers:       nil,
		Servers:      nil,
		RequestToken: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte) []byte { return []byte("i'm a invalid client") },
		ValidateRequest: func(pubkey *[tox.PUBLIC_KEY_SIZE]byte, message []byte) bool {
			_, err := verifier.Verify(pubkey[:], message)
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
		log.Fatal("Serve", zap.Error(err))
	}
}
