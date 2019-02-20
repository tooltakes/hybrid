package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/ed25519"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	ErrKeyIDLen    = errors.New("kid length not 8")
	ErrKeyAlgo     = errors.New("alg not eddsa")
	ErrKeyNotFound = errors.New("key not found")
	ErrToxRevoked  = errors.New("tox revoked")
)

type VerifyKeyer interface {
	VerifyKey(id uint32) ([]byte, bool)
}

type RevokeChecker interface {
	Revoked(id []byte) bool
}

type Verifier struct {
	VerifyKeyer
	RevokeChecker
}

func (v *Verifier) Verify(id []byte, raw []byte) (*jwt.Claims, error) {
	if v.Revoked(id) {
		return nil, ErrToxRevoked
	}

	tok, err := jwt.ParseSigned(string(raw))
	if err != nil {
		return nil, err
	}

	header := tok.Headers[0]
	if header.Algorithm != string(jose.EdDSA) {
		return nil, ErrKeyAlgo
	}

	keyid := []byte(header.KeyID)
	if len(keyid) != 8 {
		return nil, ErrKeyIDLen
	}

	_, err = hex.Decode(keyid, keyid)
	if err != nil {
		return nil, err
	}

	key, ok := v.VerifyKey(binary.BigEndian.Uint32(keyid[:4]))
	if !ok {
		return nil, ErrKeyNotFound
	}

	claims := new(jwt.Claims)
	if err = tok.Claims(ed25519.PublicKey(key), claims); err != nil {
		return nil, err
	}

	err = claims.Validate(jwt.Expected{
		Audience: jwt.Audience([]string{string(id)}),
		Time:     time.Now(),
	})
	return claims, err
}
