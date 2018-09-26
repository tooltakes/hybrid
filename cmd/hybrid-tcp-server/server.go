package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/curve25519"

	"github.com/empirefox/hybrid"
	"github.com/empirefox/hybrid/hybridauth"
	"github.com/empirefox/hybrid/hybridutils"
)

func main() {
	log, err := zap.NewDevelopment()
	defer log.Sync()
	if err != nil {
		panic(err)
	}

	c, err := LoadConfig()
	if err != nil {
		log.Fatal("LoadConfig", zap.Error(err))
	}

	if c.Port == 0 {
		log.Fatal("Post must be set")
	}

	scalar, err := hybridutils.DecodeKey32(c.ScalarHex)
	if err != nil {
		log.Fatal("DecodeSecret", zap.Error(err))
	}

	var pubkey [32]byte
	curve25519.ScalarBaseMult(&pubkey, scalar)

	verifyKey, err := hybridutils.DecodeKey32(c.VerifyKeyHex)
	if err != nil {
		log.Fatal("Parse verifyKey", zap.Error(err))
	}
	ver := Verifier(*verifyKey)
	verifier := hybridauth.Verifier{&ver, &ver}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		log.Fatal("Listen", zap.Error(err))
	}

	ln = &hybrid.CryptoListener{
		Log:      log,
		Listener: ln,
		CryptoConnServerConfig: hybrid.CryptoConnServerConfig{
			GetPrivateKey: func(serverPublic *[32]byte) (serverPrivate *[32]byte, err error) {
				if *serverPublic == pubkey {
					clone := *scalar
					return &clone, nil
				}
				return nil, fmt.Errorf("Server pubkey not found: %X", serverPublic[:])
			},
			VerifyClient: func(serverPublic, clientPublic *[32]byte, auth []byte) (interface{}, error) {
				var pk [64]byte
				hex.Encode(pk[:], clientPublic[:])
				claims, err := verifier.Verify(pk[:], auth)
				if err != nil {
					log.Info("VerifyClient", zap.ByteString("pk", pk[:]), zap.Error(err))
				}
				return claims, err
			},
			TimestampValidIn: 20,
		},
	}

	s := &hybrid.H2Server{
		Log: log,
		ReverseProxy: &httputil.ReverseProxy{
			Director:      func(req *http.Request) {},
			FlushInterval: time.Second,
		},
	}

	err = s.Serve(ln)
	if err != nil {
		log.Error("Serve", zap.Error(err))
		return
	}
}
