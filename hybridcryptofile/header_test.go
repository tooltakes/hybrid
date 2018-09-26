package hybridcryptofile

import (
	"bytes"
	"testing"
)

func TestHeader(t *testing.T) {
	h := &Header{
		Version: Version,
		BlockKB: BlockKB,

		Argon2Time:    Argon2Time,
		Argon2Memory:  Argon2Memory,
		Argon2Threads: Argon2Threads,
		Argon2Salt:    &[32]byte{},
	}

	h2, n, err := NewHeader(bytes.NewReader(h.Bytes()))
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}

	if n != HeaderLen {
		t.Errorf("should parse header len of %d, but got %d", HeaderLen, n)
	}

	if *h.Argon2Salt != *h2.Argon2Salt {
		t.Errorf("should get salt %X, but got %X", h.Argon2Salt[:], h2.Argon2Salt[:])
	}

	h2.Argon2Salt = h.Argon2Salt
	if *h2 != *h {
		t.Fatalf("should get the same header")
	}
}
