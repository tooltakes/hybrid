package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/ed25519"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

type NonceSource struct {
	Len  int
	Rand io.Reader
}

func (ns *NonceSource) Nonce() (string, error) {
	nonce := make([]byte, ns.Len)
	_, err := io.ReadFull(ns.Rand, nonce)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce), nil
}

type Signer struct {
	KeyID       uint32
	Ed25519Seed []byte
	NonceSource jose.NonceSource
	Subject     string
	Issuer      string
	Expires     time.Duration
}

type Issuer struct {
	signer  jose.Signer
	expires jwt.NumericDate
	subject string
	issuer  string
}

func NewIssuer(s *Signer) (*Issuer, error) {
	if len(s.Ed25519Seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("ed25519 seed size should not be %d", len(s.Ed25519Seed))
	}

	opts := &jose.SignerOptions{
		NonceSource: s.NonceSource,
		ExtraHeaders: map[jose.HeaderKey]interface{}{
			jose.HeaderType: "JWT",
		},
	}

	keyid := make([]byte, 4)
	binary.BigEndian.PutUint32(keyid, s.KeyID)

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.EdDSA,
		Key: jose.JSONWebKey{
			KeyID: hex.EncodeToString(keyid),
			Key:   ed25519.NewKeyFromSeed(s.Ed25519Seed),
		},
	}, opts)
	if err != nil {
		return nil, err
	}

	return &Issuer{
		signer:  signer,
		expires: jwt.NumericDate(s.Expires / time.Second),
		subject: s.Subject,
		issuer:  s.Issuer,
	}, nil
}

func (i *Issuer) Issue(claims *jwt.Claims) (string, error) {
	claims.Subject = i.subject
	claims.Issuer = i.issuer
	claims.IssuedAt = jwt.NumericDate(time.Now().Unix())
	claims.Expiry = claims.IssuedAt + i.expires
	return jwt.Signed(i.signer).Claims(claims).CompactSerialize()
}
