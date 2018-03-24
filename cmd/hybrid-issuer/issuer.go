package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/empirefox/hybrid/hybridauth"
	"golang.org/x/crypto/ed25519"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	priv    = flag.String("priv", "", "private key in hex, generate new key pair if not set")
	kid     = flag.Uint("kid", 0, "jwt header kid, must be uint")
	target  = flag.String("target", "", "curve25519 public key in hex")
	expires = flag.Uint("expires", 7, "jwt claim expires, default 7 days")
	subject = flag.String("subject", "", "jwt claim subject")
	issuer  = flag.String("issuer", "", "jwt claim issuer")
)

func main() {
	flag.Parse()

	if *priv == "" {
		pubkey, privkey, err := ed25519.GenerateKey(nil)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ed25519 PrivateKey: %x\n", privkey)
		fmt.Printf("ed25519 PublicKey: %x\n", pubkey)
		return
	}

	if *target == "" {
		log.Fatal("target should be set")
	}

	privkey, err := hex.DecodeString(*priv)
	if err != nil {
		log.Fatal(err)
	}
	if len(privkey) != ed25519.PrivateKeySize {
		log.Fatalf("private key should be a 128 size hex, but got %d", len(privkey))
	}

	i, err := hybridauth.NewIssuer(&hybridauth.Signer{
		KeyID: uint32(*kid),
		Key:   ed25519.PrivateKey(privkey),
		NonceSource: &hybridauth.NonceSource{
			Len:  24,
			Rand: rand.Reader,
		},
		Subject: *subject,
		Issuer:  *issuer,
		Expires: time.Duration(*expires) * time.Hour * 24,
	})
	if err != nil {
		log.Fatal(err)
	}

	jwtStr, err := i.Issue(&jwt.Claims{
		Audience: jwt.Audience([]string{strings.ToLower(*target)}),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("issue ok:")
	fmt.Println(jwtStr)
}
