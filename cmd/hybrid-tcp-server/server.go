package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/empirefox/hybrid"
)

var (
	clientPubHex = strings.ToLower("F447C5472BDA9AC0DC98ACFE0E40D1434CC215CCF9D1729542A787BD0AEC5432")
	serverPubHex = strings.ToLower("9AA0FF6C243F90947035FA4AA45353A705B4F9681299AC61A295E3C32911EB63")

	serverScalar    [32]byte
	serverScalarHex = []byte(strings.ToLower("CF43FFE81487EA74A519C568E5D2CD79611D3661919617B9D8E542F4ECAB8977"))
)

func main() {
	log, err := zap.NewProduction()
	defer log.Sync()
	if err != nil {
		panic(err)
	}

	_, err = hex.Decode(serverScalar[:], serverScalarHex)
	if err != nil {
		log.Error("serverScalar", zap.Error(err))
		return
	}

	cryptoConfig := &hybrid.CryptoConnServerConfig{
		GetPrivateKey: GetPrivateKey,
	}

	s := &hybrid.Server{
		Log:             log,
		ListenerCreator: hybrid.NewTCPListenerCreator(":19999", cryptoConfig),
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

func GetPrivateKey(serverPublic, clientPublic []byte) (serverPrivate *[32]byte, err error) {
	if hex.EncodeToString(clientPublic) == clientPubHex && hex.EncodeToString(serverPublic) == serverPubHex {
		// serverScalar will be reused
		clone := serverScalar
		return &clone, nil
	}
	fmt.Println(hex.EncodeToString(clientPublic), "==", clientPubHex)
	fmt.Println(hex.EncodeToString(serverPublic), "==", serverPubHex)
	return nil, errors.New("serverPrivate not found")
}
