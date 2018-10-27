// visit client:
// http://0.hybrid/index.html
//
// visit server:
// http://server_name_in_hybrid_json.hybrid/index.html
//
// visit host from special server:
// http://192.168.2.6.server_name_in_hybrid_json.hybrid/index.html
//
// ssh over hybrid example:
// ssh root@192.168.1.1 -o "ProxyCommand=nc -n -Xconnect -x127.0.0.1:7777 %h.server_name_in_hybrid_json.hybrid %p"
package main

import (
	"context"
	"net/http"
	"os"

	"github.com/empirefox/hybrid/hybridauth"
	"github.com/empirefox/hybrid/hybridclient"
	"github.com/empirefox/hybrid/hybridutils"
	"go.uber.org/zap"

	ipfsConfig "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-config"
)

type Verifier struct {
	// public key
	ed25519key []byte
	revoked    map[string]bool
}

func NewVerifier(ed25519key []byte, revoked []string) *Verifier {
	verifier := Verifier{
		ed25519key: ed25519key,
		revoked:    make(map[string]bool, len(revoked)),
	}
	for _, r := range revoked {
		verifier.revoked[r] = true
	}
	return &verifier
}

func (v *Verifier) VerifyKey(id uint32) ([]byte, bool) { return v.ed25519key, true }
func (v *Verifier) Revoked(id []byte) bool             { return v.revoked[string(id)] }

func main() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	config, err := hybridclient.LoadConfig(nil)
	if err != nil {
		log.Fatal("LoadConfig", zap.Error(err))
	}

	t, err := config.ConfigTree()
	if err != nil {
		log.Fatal("ConfigTree", zap.Error(err))
	}
	os.Setenv(ipfsConfig.EnvDir, t.IpfsPath)

	ed25519key, err := hybridutils.DecodeKey32(config.Ipfs.VerifyKeyHex)
	if err != nil {
		log.Fatal("Ipfs.VerifyKeyHex", zap.Error(err))
	}

	// verify using ed25519key
	ver := NewVerifier(ed25519key[:], config.Ipfs.Revoked)
	verifier := hybridauth.Verifier{ver, ver}
	verify := func(peerID, token []byte) bool {
		_, err := verifier.Verify(peerID, token)
		if err != nil {
			log.Info("Verify", zap.ByteString("peerID", peerID), zap.Error(err))
		}
		return err == nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hi, err := hybridclient.NewIpfs(ctx, config, verify, log)
	if err != nil {
		log.Fatal("NewIpfs", zap.Error(err))
	}

	err = hi.Connect()
	if err != nil {
		log.Fatal("ipfs.Connect", zap.Error(err))
	}

	localServers := map[string]http.Handler{}
	client, err := hybridclient.NewClient(config, hi, localServers, nil, log)
	if err != nil {
		log.Fatal("NewClient", zap.Error(err))
	}
	defer client.Close()

	err = client.Serve()
	if err != nil {
		log.Fatal("Serve", zap.Error(err))
	}
}
