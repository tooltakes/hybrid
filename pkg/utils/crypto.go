package utils

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

const (
	Key32Size = 32
)

func DecodeKey32(fromHex string) (*[Key32Size]byte, error) {
	var key [Key32Size]byte
	n, err := hex.Decode(key[:], bytes.ToLower([]byte(fromHex)))
	if err != nil {
		return nil, err
	}
	if n != Key32Size {
		return nil, fmt.Errorf("key bytes len should be %d, but got %d", Key32Size, n)
	}
	return &key, nil
}
