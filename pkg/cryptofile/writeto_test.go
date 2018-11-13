package cryptofile

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

type dataInfo struct {
	size int
}

var dataInfos = []dataInfo{
	{1},
	{2},
	{HeaderLen - 1},
	{HeaderLen},
	{HeaderLen + 1},
	{BlockKB<<10 - 1},
	{BlockKB << 10},
	{BlockKB<<10 + 1},
	{HeaderLen + BlockKB<<10 - 1},
	{HeaderLen + BlockKB<<10},
	{HeaderLen + BlockKB<<10 + 1},
	{HeaderLen + BlockKB<<10 + 2},
	{1 << 20},
}

func TestCryptOk(t *testing.T) {
	password := make([]byte, 32)
	plainbuf := bytes.NewBuffer(nil)
	cipherbuf := bytes.NewBuffer(nil)

	for _, di := range dataInfos {
		_, err := io.ReadFull(rand.Reader, password)
		if err != nil {
			t.Fatalf("Failed to generate random key: %v", err)
		}

		plaintext := make([]byte, di.size)
		_, err = io.ReadFull(rand.Reader, plaintext)
		if err != nil {
			t.Fatalf("Failed to generate random plaintext: %v", err)
		}

		wt, err := NewEncryptWriterTo(CryptoConfig{Password: password}, nil, bytes.NewReader(plaintext))
		if err != nil {
			t.Fatalf("Failed to NewEncryptWriterTo: %v", err)
		}

		cipherbuf.Reset()
		nn, err := wt.WriteTo(cipherbuf)
		if err != nil {
			t.Fatalf("Failed to encrypt plaintext: %v", err)
		}

		if nn != int64(di.size) {
			t.Fatalf("Should get plaintext size of %d, but got %d", di.size, nn)
		}

		cipherlen := int64(cipherbuf.Len())

		wt, n, err := NewDecryptWriterTo(CryptoConfig{Password: password}, cipherbuf)
		if err != nil {
			t.Fatalf("Failed to NewDecryptWriterTo: %v", err)
		}

		if n != HeaderLen {
			t.Fatalf("Should get cipherbuf header size of %d, but got %d", HeaderLen, n)
		}

		plainbuf.Reset()
		nn, err = wt.WriteTo(plainbuf)
		if err != nil {
			t.Fatalf("Failed to decrypt ciphertext: %v", err)
		}

		if nn != cipherlen {
			t.Fatalf("Should get ciphertext size of %d, but got %d", cipherlen, nn)
		}

		if bytes.Compare(plainbuf.Bytes(), plaintext) != 0 {
			t.Fatalf("Should get ciphertext %X, but got %X", plaintext, plainbuf.Bytes())
		}
	}
}
