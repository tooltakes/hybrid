package main

import (
	"encoding/hex"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

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

	aeskey, err := hex.DecodeString(strings.ToLower(key))
	if err != nil {
		log.Error("DecodeString", zap.Error(err))
	}
	block, err := kcp.NewAESBlockCrypt(aeskey)
	if err != nil {
		log.Error("NewAESBlockCrypt", zap.Error(err))
	}

	s := &hybrid.Server{
		Log:             log,
		ListenerCreator: hybridkcp.NewKCPListenerCreator(block, ":19995"),
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
