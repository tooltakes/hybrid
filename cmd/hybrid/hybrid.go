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
	"sync"

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
	hi, listensErrc, err := hybridclient.NewIpfs(ctx, config, verify, log)
	if err != nil {
		log.Fatal("NewIpfs", zap.Error(err))
	}

	err = hi.Connect()
	if err != nil {
		log.Fatal("ipfs.Connect", zap.Error(err))
	}

	var clientErrc <-chan error
	if config.Bind != "" {
		localServers := map[string]http.Handler{}
		client, err := hybridclient.NewClient(config, hi, localServers, nil, log)
		if err != nil {
			log.Fatal("NewClient", zap.Error(err))
		}
		defer client.Close()
		clientErrc = client.StartServe()
	}

	for err := range merge(clientErrc, listensErrc) {
		if err != nil {
			log.Error("hybrid exit", zap.Error(err))
		}
	}
	err = hi.Proccess().Err()
	if err != nil {
		log.Error("hybrid exit", zap.Error(err))
	}
}

// merge does fan-in of multiple read-only error channels
// taken from http://blog.golang.org/pipelines
func merge(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan error) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	for _, c := range cs {
		if c != nil {
			wg.Add(1)
			go output(c)
		}
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
