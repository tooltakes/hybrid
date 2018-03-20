package hybridauth

import (
	"encoding/binary"
	"errors"
	"time"

	"golang.org/x/crypto/ed25519"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	ErrKeyIDLen    = errors.New("kid length not 4")
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

func (v *Verifier) Verify(pk []byte, raw []byte) (*jwt.Claims, error) {
	if v.Revoked(pk) {
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
	if len(keyid) != 4 {
		return nil, ErrKeyIDLen
	}

	key, ok := v.VerifyKey(binary.BigEndian.Uint32(keyid))
	if !ok {
		return nil, ErrKeyNotFound
	}

	claims := new(jwt.Claims)
	if err = tok.Claims(ed25519.PublicKey(key), claims); err != nil {
		return nil, err
	}

	err = claims.Validate(jwt.Expected{
		Audience: jwt.Audience([]string{string(pk)}),
		Time:     time.Now(),
	})
	return claims, err
}
