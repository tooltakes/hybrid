package auth

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ed25519"
	"gopkg.in/square/go-jose.v2/jwt"
)

type reader struct{}

func (r *reader) Read(p []byte) (n int, err error) {
	return len(p), nil
}

func newGetKeyFunc(id uint64) GetKeyFunc {
	valid := make([]byte, 8)
	binary.BigEndian.PutUint64(valid, id)
	return func(keyid []byte) (key []byte, err error) {
		if bytes.Equal(keyid, valid) {
			return pub, nil
		}
		return nil, fmt.Errorf("valid: %x, but got: %x", valid, keyid)
	}
}

func decodeKey(s string) []byte {
	sk, _ := hex.DecodeString(strings.ToLower(s))
	return sk
}

var (
	priv0 = decodeKey("0000000000000000000000000000000000000000000000000000000000000000")
	pub   = decodeKey("3B6A27BCCEB6A42D62A3A8D02A6F0D73653215771DE243A63AC048A18B59DA29")
	priv  = ed25519.PrivateKey(append(priv0, pub...))
)

func TestAuth(t *testing.T) {
	s := &Signer{
		KeyID: KeyIDFromUint64(100),
		Key:   priv,
		NonceSource: &NonceSource{
			Len:  32,
			Rand: new(reader), // 0000...
		},
		Subject: "opreate",
		Issuer:  "hybrid",
		Expires: 200 * time.Second,
	}

	issuer, err := NewIssuer(s)
	if err != nil {
		t.Errorf("should create issuer: %v", err)
		return
	}
	claims := &jwt.Claims{
		Audience: jwt.Audience([]string{"toxpub1"}),
	}
	tok, err := issuer.Issue(claims)
	if err != nil {
		t.Errorf("should issue ok: %v", err)
		return
	}

	allok := newGetKeyFunc(100)
	claims, err = allok.Verify([]byte("toxpub1"), []byte(tok))
	if err != nil {
		t.Errorf("should Verify ok: %v", err)
		return
	}

	if claims.Issuer != "hybrid" {
		t.Errorf("claims issuer should be hybrid, but got %s", claims.Issuer)
		return
	}

	fail := newGetKeyFunc(100)
	claims, err = fail.Verify([]byte("toxpub2"), []byte(tok))
	if err == nil {
		t.Errorf("should Verify fail")
		return
	}

	fail = newGetKeyFunc(101)
	claims, err = fail.Verify([]byte("toxpub1"), []byte(tok))
	if err == nil {
		t.Errorf("should Verify fail")
		return
	}
}
