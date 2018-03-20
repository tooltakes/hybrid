package main

type Config struct {
	ToxSecretHex string
	ToxNospam    uint32
	VerifyKeyHex string
}

type Verifier [32]byte

func (v *Verifier) VerifyKey(id uint32) ([]byte, bool) { return v[:], true }
func (v *Verifier) Revoked(id []byte) bool             { return false }
