package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/empirefox/hybrid/pkg/auth"
	"golang.org/x/crypto/ed25519"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	seedHex = flag.String("seed", "", "seed in hex, generate new key pair if not set")
	kid     = flag.Uint("kid", 0, "jwt header kid, must be uint")
	target  = flag.String("target", "", "target id in hex")
	expires = flag.Uint("expires", 7, "jwt claim expires, default 7 days")
	subject = flag.String("subject", "", "jwt claim subject")
	issuer  = flag.String("issuer", "", "jwt claim issuer")
)

func main() {
	flag.Parse()

	if *seedHex == "" {
		_, privkey, err := ed25519.GenerateKey(nil)
		if err != nil {
			log.Fatal(err)
		}
		printJson(privkey)
		return
	}

	if *target == "" {
		log.Fatal("target should be set")
	}

	seed, err := hex.DecodeString(*seedHex)
	if err != nil {
		log.Fatal(err)
	}
	if len(seed) != ed25519.SeedSize {
		log.Fatalf("seed should be a 64 size hex, but got %d", len(seed))
	}

	i, err := hybridauth.NewIssuer(&hybridauth.Signer{
		KeyID:       uint32(*kid),
		Ed25519Seed: seed,
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
		Audience: jwt.Audience([]string{*target}),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("issue ok:")
	fmt.Println(jwtStr)
}

func printJson(privkey ed25519.PrivateKey) {
	fmt.Printf(`{
  "Seed": "%x",
  "Pubkey": "%x"
}
`, privkey.Seed(), privkey.Public())
}
