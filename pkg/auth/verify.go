package auth

import (
	"encoding/base64"
	"errors"
	"time"

	"golang.org/x/crypto/ed25519"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	ErrKeyAlgo = errors.New("alg not eddsa")
)

type GetKeyFunc func(keyid []byte) (key []byte, err error)

// Verify decodes keyid from base64
func (getKey GetKeyFunc) Verify(id []byte, raw []byte) (*jwt.Claims, error) {
	tok, err := jwt.ParseSigned(string(raw))
	if err != nil {
		return nil, err
	}

	header := tok.Headers[0]
	if header.Algorithm != string(jose.EdDSA) {
		return nil, ErrKeyAlgo
	}

	keyid, err := base64.RawURLEncoding.DecodeString(header.KeyID)
	if err != nil {
		return nil, err
	}

	key, err := getKey(keyid)
	if err != nil {
		return nil, err
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
