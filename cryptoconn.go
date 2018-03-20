package hybrid

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"time"

	"github.com/aead/poly1305"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// cryptoPayloadSizeMask is the maximum size of payload in bytes.
const cryptoPayloadSizeMask = 0x3FFF // 16*1024 - 1

var (
	ErrVerifyClientHello = errors.New("Verify client_hello failed")
)

type cryptoWriter struct {
	io.Writer
	cipher.AEAD
	noncer *noncer
	buf    []byte
}

// NewCryptoWriter wraps an io.Writer with AEAD encryption.
func NewCryptoWriter(w io.Writer, aead cipher.AEAD, seed512 []byte) (io.Writer, error) {
	return newCryptoWriter(w, aead, seed512)
}

func newCryptoWriter(w io.Writer, aead cipher.AEAD, seed512 []byte) (*cryptoWriter, error) {
	n, err := NewFixedEndlessNoncer(seed512)
	if err != nil {
		return nil, err
	}
	return &cryptoWriter{
		Writer: w,
		AEAD:   aead,
		buf:    make([]byte, 2+aead.Overhead()+cryptoPayloadSizeMask+aead.Overhead()),
		noncer: n,
	}, nil
}

// Write encrypts b and writes to the embedded io.Writer.
func (w *cryptoWriter) Write(b []byte) (int, error) {
	n, err := w.ReadFrom(bytes.NewBuffer(b))
	return int(n), err
}

// ReadFrom reads from the given io.Reader until EOF or error, encrypts and
// writes to the embedded io.Writer. Returns number of bytes read from r and
// any error encountered.
// TODO remove block of r.Read
func (w *cryptoWriter) ReadFrom(r io.Reader) (n int64, err error) {
	for {
		buf := w.buf
		payloadBuf := buf[2+w.Overhead() : 2+w.Overhead()+cryptoPayloadSizeMask]
		nr, er := r.Read(payloadBuf)

		if nr > 0 {
			n += int64(nr)
			buf = buf[:2+w.Overhead()+nr+w.Overhead()]
			payloadBuf = payloadBuf[:nr]
			buf[0], buf[1] = byte(nr>>8), byte(nr) // big-endian payload size
			w.Seal(buf[:0], w.noncer.Next(), buf[:2], nil)

			w.Seal(payloadBuf[:0], w.noncer.Next(), payloadBuf, nil)

			_, ew := w.Writer.Write(buf)
			if ew != nil {
				err = ew
				break
			}
		}

		if er != nil {
			if er != io.EOF { // ignore EOF as per io.ReaderFrom contract
				err = er
			}
			break
		}
	}

	return n, err
}

type cryptoReader struct {
	io.Reader
	cipher.AEAD
	noncer   *noncer
	buf      []byte
	leftover []byte
}

// NewCryptoReader wraps an io.Reader with AEAD decryption.
func NewCryptoReader(r io.Reader, aead cipher.AEAD, seed512 []byte) (io.Reader, error) {
	return newCryptoReader(r, aead, seed512)
}

func newCryptoReader(r io.Reader, aead cipher.AEAD, seed512 []byte) (*cryptoReader, error) {
	n, err := NewFixedEndlessNoncer(seed512)
	if err != nil {
		return nil, err
	}
	return &cryptoReader{
		Reader: r,
		AEAD:   aead,
		buf:    make([]byte, cryptoPayloadSizeMask+aead.Overhead()),
		noncer: n,
	}, nil
}

// read and decrypt a record into the internal buffer. Return decrypted payload length and any error encountered.
func (r *cryptoReader) read() (int, error) {
	// decrypt payload size
	buf := r.buf[:2+r.Overhead()]
	_, err := io.ReadFull(r.Reader, buf)
	if err != nil {
		return 0, err
	}

	_, err = r.Open(buf[:0], r.noncer.Next(), buf, nil)
	if err != nil {
		return 0, err
	}

	size := (int(buf[0])<<8 + int(buf[1])) & cryptoPayloadSizeMask

	// decrypt payload
	buf = r.buf[:size+r.Overhead()]
	_, err = io.ReadFull(r.Reader, buf)
	if err != nil {
		return 0, err
	}

	_, err = r.Open(buf[:0], r.noncer.Next(), buf, nil)
	if err != nil {
		return 0, err
	}

	return size, nil
}

// Read reads from the embedded io.Reader, decrypts and writes to b.
func (r *cryptoReader) Read(b []byte) (int, error) {
	// copy decrypted bytes (if any) from previous record first
	if len(r.leftover) > 0 {
		n := copy(b, r.leftover)
		r.leftover = r.leftover[n:]
		return n, nil
	}

	n, err := r.read()
	m := copy(b, r.buf[:n])
	if m < n { // insufficient len(b), keep leftover for next read
		r.leftover = r.buf[m:n]
	}
	return m, err
}

// WriteTo reads from the embedded io.Reader, decrypts and writes to w until
// there's no more data to write or when an error occurs. Return number of
// bytes written to w and any error encountered.
func (r *cryptoReader) WriteTo(w io.Writer) (n int64, err error) {
	// write decrypted bytes left over from previous record
	for len(r.leftover) > 0 {
		nw, ew := w.Write(r.leftover)
		r.leftover = r.leftover[nw:]
		n += int64(nw)
		if ew != nil {
			return n, ew
		}
	}

	for {
		nr, er := r.read()
		if nr > 0 {
			nw, ew := w.Write(r.buf[:nr])
			n += int64(nw)

			if ew != nil {
				err = ew
				break
			}
		}

		if er != nil {
			if er != io.EOF { // ignore EOF as per io.Copy contract (using src.WriteTo shortcut)
				err = er
			}
			break
		}
	}

	return n, err
}

type noncer struct {
	// must be len of blake2b.Size+chacha20poly1305.NonceSize-1
	seed []byte
	// [0, blake2b.Size)
	p int
}

func NewFixedEndlessNoncer(seed512 []byte) (*noncer, error) {
	if len(seed512) != blake2b.Size {
		return nil, fmt.Errorf("noncer seed must be len of %d, but got %d", blake2b.Size, len(seed512))
	}
	n := noncer{seed: make([]byte, blake2b.Size+chacha20poly1305.NonceSize-1)}
	n.reset(seed512)
	return &n, nil
}

// Next must be used before the next call, since noncer may reuse the result bytes
func (n *noncer) Next() []byte {
	if n.p == blake2b.Size {
		seed512 := blake2b.Sum512(n.seed[:blake2b.Size])
		n.reset(seed512[:])
	}
	nonce := n.seed[n.p : n.p+chacha20poly1305.NonceSize]
	n.p++
	return nonce
}

func (n *noncer) reset(seed512 []byte) {
	n.p = 0
	copy(n.seed[:blake2b.Size], seed512)
	copy(n.seed[blake2b.Size:], n.seed[:chacha20poly1305.NonceSize-1])
}

type cryptoStreamConn struct {
	*cryptoReader
	*cryptoWriter
	c net.Conn
}

func (c *cryptoStreamConn) Underline() net.Conn                { return c.c }
func (c *cryptoStreamConn) Close() error                       { return c.c.Close() }
func (c *cryptoStreamConn) LocalAddr() net.Addr                { return c.c.LocalAddr() }
func (c *cryptoStreamConn) RemoteAddr() net.Addr               { return c.c.RemoteAddr() }
func (c *cryptoStreamConn) SetDeadline(t time.Time) error      { return c.c.SetDeadline(t) }
func (c *cryptoStreamConn) SetReadDeadline(t time.Time) error  { return c.c.SetReadDeadline(t) }
func (c *cryptoStreamConn) SetWriteDeadline(t time.Time) error { return c.c.SetWriteDeadline(t) }

type CryptoConnServerConfig struct {
	Rand          io.Reader
	GetPrivateKey func(serverPublic, clientPublic []byte) (serverPrivate *[32]byte, err error)
}

// TODO use tls1.3
func NewCryptoServerConn(c net.Conn, config *CryptoConnServerConfig) (net.Conn, error) {
	if _, ok := c.(*cryptoStreamConn); ok {
		return c, nil
	}

	if config.GetPrivateKey == nil {
		return nil, errors.New("CryptoConnServerConfig: GetPrivateKey must be set")
	}
	// handshake
	//
	// client_hello len=176 chacha20poly1305=main_shared_key
	// server_pub(32) client_pub(32) nonce(12) chacha20poly1305{crc32_pubs_nonce(4) next_client_pub(32) nonce_seed_a(64) poly1305(16)}
	//
	// server_hello len=108 chacha20poly1305=main_shared_key
	// nonce(12) chacha20poly1305{next_server_pub(32) nonce_seed_b(64)}

	randReader := config.Rand
	if randReader == nil {
		randReader = rand.Reader
	}

	clientHelloLen := 176 + poly1305.TagSize

	// 1. read client_hello
	clientHello := make([]byte, clientHelloLen)
	_, err := io.ReadFull(c, clientHello)
	if err != nil {
		return nil, err
	}

	// 2. find main private_key
	clientPublic := clientHello[32:64]
	scalar, err := config.GetPrivateKey(clientHello[:32], clientPublic)
	if err != nil {
		return nil, err
	}

	var pub, sharedKey [32]byte
	// 3. main sharedKey
	// theirPublic
	copy(pub[:], clientPublic)
	curve25519.ScalarMult(&sharedKey, scalar, &pub)

	// 4. main aead
	aead0, err := chacha20poly1305.New(sharedKey[:])
	if err != nil {
		return nil, err
	}

	// 5. decrypt text
	// crc32_pubs_nonce(4) next_client_pub(32) nonce_seed_a(64)
	// client send:
	// aead0.Seal(clientHello[76:76], clientHello[64:76], clientHello[76:], nil)
	clientTxt, err := aead0.Open(clientHello[76:76], clientHello[64:76], clientHello[76:], nil)
	if err != nil {
		return nil, err
	}
	var nextClientPub [32]byte
	copy(nextClientPub[:], clientTxt[4:36])

	// 6. verify checksum
	if crc32.ChecksumIEEE(clientHello[:76]) != binary.BigEndian.Uint32(clientTxt[:4]) {
		return nil, ErrVerifyClientHello
	}

	// 7. Generate next_server curve25519 for shared_key
	// reuse scalar as next_server_priv
	_, err = io.ReadFull(randReader, scalar[:])
	if err != nil {
		return nil, err
	}
	// reuse pub as next_server_pub
	curve25519.ScalarBaseMult(&pub, scalar)

	// 8. server_hello(text) len=108 chacha20poly1305=main_shared_key
	// nonce(12) chacha20poly1305{next_server_pub(32) nonce_seed_b(64)}
	// reuse clientHello
	serverHello := clientHello[:108+poly1305.TagSize]
	// 8.1 nonce
	_, err = io.ReadFull(randReader, serverHello[:12])
	if err != nil {
		return nil, err
	}
	// 8.2 next_server_pub
	copy(serverHello[12:44], pub[:])
	// 8.1 nonce_seed_b
	_, err = io.ReadFull(randReader, serverHello[44:108])
	if err != nil {
		return nil, err
	}

	// 9. get nonce_seed before encrypt
	// nonce_seed_a will be copied with poly1305
	// nonce_seed_b will be encrypted
	// reuse pub, sharedKey as buf32
	a2b, b2a := newNonceSeed(clientTxt[36:], serverHello[44:108], pub[:], sharedKey[:], true)

	// 10. encrypt_send server_hello
	aead0.Seal(serverHello[12:12], serverHello[:12], serverHello[12:108], nil)
	_, err = c.Write(serverHello)
	if err != nil {
		return nil, err
	}

	// 11. session aead
	curve25519.ScalarMult(&sharedKey, scalar, &nextClientPub)
	aead, err := chacha20poly1305.New(sharedKey[:])
	if err != nil {
		return nil, err
	}

	// 12. create conn
	r, err := newCryptoReader(c, aead, a2b)
	if err != nil {
		return nil, err
	}
	w, err := newCryptoWriter(c, aead, b2a)
	if err != nil {
		return nil, err
	}

	return &cryptoStreamConn{cryptoReader: r, cryptoWriter: w, c: c}, nil
}

type CryptoConnConfig struct {
	Rand         io.Reader
	ServerPublic *[32]byte
	ClientScalar *[32]byte
	// GetClientScalar get scalar if ClientScalar not set
	GetClientScalar func(c net.Conn, serverPublic *[32]byte) (*[32]byte, error)
}

// NewCryptoConn wraps a stream-oriented net.Conn for client.
// TODO use tls1.3
func NewCryptoConn(c net.Conn, config *CryptoConnConfig) (net.Conn, error) {
	if _, ok := c.(*cryptoStreamConn); ok {
		return c, nil
	}

	if config.ServerPublic == nil {
		return nil, errors.New("CryptoConnConfig: ServerPublic must be set")
	}

	var err error
	var scalar *[32]byte
	if config.ClientScalar != nil {
		// to reuse scalar
		scalar0 := *config.ClientScalar
		scalar = &scalar0
	} else {
		if config.GetClientScalar == nil {
			return nil, errors.New("CryptoConnConfig: ClientScalar or GetClientScalar must be set")
		}
		scalar, err = config.GetClientScalar(c, config.ServerPublic)
		if err != nil {
			return nil, err
		}
	}

	randReader := config.Rand
	if randReader == nil {
		randReader = rand.Reader
	}

	// handshake
	//
	// client_hello len=176 chacha20poly1305=main_shared_key
	// server_pub(32) client_pub(32) nonce(12) chacha20poly1305{crc32_pubs_nonce(4) next_client_pub(32) nonce_seed_a(64) poly1305(16}
	//
	// server_hello len=108 chacha20poly1305=main_shared_key
	// nonce(12) chacha20poly1305{next_server_pub(32) nonce_seed_b(64)}

	var pub, sharedKey [32]byte
	// 1. aead0 with main sharedKey
	curve25519.ScalarBaseMult(&pub, scalar)
	curve25519.ScalarMult(&sharedKey, scalar, config.ServerPublic)
	aead0, err := chacha20poly1305.New(sharedKey[:])
	if err != nil {
		return nil, err
	}

	clientHelloLen := 176 + poly1305.TagSize
	buf := make([]byte, clientHelloLen+64)
	clientNonceSeed := buf[clientHelloLen:]
	_, err = io.ReadFull(randReader, clientNonceSeed)
	if err != nil {
		return nil, err
	}

	// 2. build client_hello
	clientHello := buf[:clientHelloLen]
	// 2.1 server_pub
	copy(clientHello[:32], config.ServerPublic[:])
	// 2.2 client_pub
	copy(clientHello[32:64], pub[:])
	// 2.2 nonce
	_, err = io.ReadFull(randReader, clientHello[64:76])
	if err != nil {
		return nil, err
	}
	// 2.3 crc32_pubs_nonce
	binary.BigEndian.PutUint32(clientHello[76:80], crc32.ChecksumIEEE(clientHello[:76]))
	// 2.4 next_client_pub
	// reuse scalar as next_client_priv
	_, err = io.ReadFull(randReader, scalar[:])
	if err != nil {
		return nil, err
	}
	// reuse pub as next_client_pub
	curve25519.ScalarBaseMult(&pub, scalar)
	copy(clientHello[80:112], pub[:])
	// 2.5 nonce_seed_a
	copy(clientHello[112:176], clientNonceSeed)

	// 3. send encrypt
	aead0.Seal(clientHello[76:76], clientHello[64:76], clientHello[76:176], nil)
	_, err = c.Write(clientHello)
	if err != nil {
		return nil, err
	}

	// 4. get server_hello
	serverHello := clientHello[:108+poly1305.TagSize]
	_, err = io.ReadFull(c, serverHello)
	if err != nil {
		return nil, err
	}

	// 5. decrypt server_hello
	// server_hello len=108 chacha20poly1305=main_shared_key
	// nonce(12) chacha20poly1305{next_server_pub(32) nonce_seed_b(64)}
	// serverTxt: next_server_pub(32) nonce_seed_b(64)
	serverTxt, err := aead0.Open(serverHello[12:12], serverHello[:12], serverHello[12:], nil)
	if err != nil {
		return nil, err
	}

	// 6. session aead
	// reuse pub as next_server_pub
	copy(pub[:], serverTxt[:32])
	curve25519.ScalarMult(&sharedKey, scalar, &pub)
	aead, err := chacha20poly1305.New(sharedKey[:])
	if err != nil {
		return nil, err
	}

	// 7. nonce_seed
	// reuse pub, sharedKey as buf32
	a2b, b2a := newNonceSeed(clientNonceSeed, serverTxt[32:], pub[:], sharedKey[:], false)

	// 8. create conn
	r, err := newCryptoReader(c, aead, b2a)
	if err != nil {
		return nil, err
	}
	w, err := newCryptoWriter(c, aead, a2b)
	if err != nil {
		return nil, err
	}

	return &cryptoStreamConn{cryptoReader: r, cryptoWriter: w, c: c}, nil
}

func newNonceSeed(a, b, buf32a, buf32b []byte, restore bool) (a2b, b2a []byte) {
	// save a2
	copy(buf32a, a[32:])
	// a1+b2
	copy(a[32:], b[32:])
	a2b_ := blake2b.Sum512(a)
	if restore {
		// restore a2
		copy(a[32:], buf32a)
		// save b2
		copy(buf32b, b[32:])
	}
	// b1+a2
	copy(b[32:], buf32a)
	b2a_ := blake2b.Sum512(b)
	if restore {
		// restore b2
		copy(b[32:], buf32b)
	}
	return a2b_[:], b2a_[:]
}
