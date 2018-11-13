package cryptofile

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"github.com/aead/poly1305"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	Version       = 1
	BlockKB       = 255
	Argon2Time    = 4
	Argon2Memory  = 32 << 10
	Argon2Threads = 4

	BlockQueue = 8
	HeaderLen  = 44
)

var (
	ErrEmptyPassword = errors.New("empty password")
)

type CryptoConfig struct {
	// Password the only required field here.
	Password []byte

	BlockQueue uint8

	// GetBufferPool use NewBufferPool if nil.
	GetBufferPool func(block int) *sync.Pool
}

// Header Format
//+----------------------------------------------------------------------------------+
//| Version | block(KB) | argon2Time | argon2Memory(KB) | argon2Threads | argon2Salt |
//+----------------------------------------------------------------------------------+
//    (16)      (8)          (32)           (32)               (8)           (256)
type Header struct {
	// Version only support exactly the same version.
	Version uint16

	// BlockKB plaintext size block size.
	BlockKB uint8

	Argon2Time    uint32
	Argon2Memory  uint32
	Argon2Threads uint8
	Argon2Salt    *[32]byte
}

func NewHeader(r io.Reader) (*Header, int, error) {
	var head [HeaderLen]byte
	n, err := io.ReadFull(r, head[:])
	if err != nil {
		return nil, n, err
	}

	var salt [32]byte
	copy(salt[:], head[12:])
	return &Header{
		Version:       binary.BigEndian.Uint16(head[:]),  // +2=2
		BlockKB:       uint8(head[2]),                    // +1=3
		Argon2Time:    binary.BigEndian.Uint32(head[3:]), // +4=7
		Argon2Memory:  binary.BigEndian.Uint32(head[7:]), // +4=11
		Argon2Threads: uint8(head[11]),                   // +1=12
		Argon2Salt:    &salt,                             // +32=44
	}, n, nil
}

func (h *Header) Bytes() []byte {
	var head [HeaderLen]byte
	binary.BigEndian.PutUint16(head[:], h.Version)       // +2=2
	head[2] = byte(h.BlockKB)                            // +1=3
	binary.BigEndian.PutUint32(head[3:], h.Argon2Time)   // +4=7
	binary.BigEndian.PutUint32(head[7:], h.Argon2Memory) // +4=11
	head[11] = byte(h.Argon2Threads)                     // +1=12
	copy(head[12:], h.Argon2Salt[:])                     // +32=44
	return head[:]
}

func (h *Header) KeyDerive(password []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, errors.New("password must be set")
	}
	return argon2.IDKey(
		password,
		h.Argon2Salt[:],
		h.Argon2Time,
		h.Argon2Memory,
		h.Argon2Threads,
		32,
	), nil
}

func NewBufferPool(block int) *sync.Pool {
	return &sync.Pool{New: func() interface{} { return newBuffer(block) }}
}

type buffer struct {
	block      int
	full       []byte
	dst        []byte
	nonce      []byte
	plaintext  []byte
	ciphertext []byte
	src        []byte
	result     []byte
	short      bool
	encrypt    bool
}

func newBuffer(block int) *buffer {
	// | nonce | src | poly1305 |
	full := make([]byte, chacha20poly1305.NonceSize+block+poly1305.TagSize)
	return &buffer{
		block:      block,
		full:       full,
		dst:        full[chacha20poly1305.NonceSize:chacha20poly1305.NonceSize],
		nonce:      full[:chacha20poly1305.NonceSize],
		plaintext:  full[chacha20poly1305.NonceSize : chacha20poly1305.NonceSize+block],
		ciphertext: full[chacha20poly1305.NonceSize:],
		short:      false,
	}
}

func (buf *buffer) Init(encrypt bool) *buffer {
	buf.encrypt = encrypt
	if buf.short {
		return buf.WithRead(0)
	}
	return buf.initsrc()
}

func (buf *buffer) initsrc() *buffer {
	if buf.encrypt {
		buf.src = buf.plaintext
		buf.result = buf.ciphertext
	} else {
		buf.src = buf.ciphertext
		buf.result = buf.plaintext
	}
	return buf
}

func (buf *buffer) WithRead(n int) *buffer {
	block := n
	if !buf.encrypt {
		block -= poly1305.TagSize
	}
	if block <= 0 {
		block = buf.block
	}
	buf.short = block != buf.block

	buf.plaintext = buf.full[chacha20poly1305.NonceSize : chacha20poly1305.NonceSize+block]
	buf.ciphertext = buf.full[chacha20poly1305.NonceSize : chacha20poly1305.NonceSize+block+poly1305.TagSize]
	return buf.initsrc()
}
