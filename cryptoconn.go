package hybrid

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// cryptoPayloadSizeMask is the maximum size of payload in bytes.
const (
	cryptoPayloadSizeMask = 0x3FFF // 16*1024 - 1
	handshakeVersion      = 0x6a01

	MaxAuthTokenSize = 732
)

var (
	ErrVerifyClientHello = errors.New("Verify client_hello failed")
)

func argon2KeyDerive(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, 3, 16<<10, 2, 32)
}

func curve25519ScalarMult(dst, in, base *[32]byte, salt []byte) {
	curve25519.ScalarMult(dst, in, base)
	key := argon2KeyDerive(dst[:], salt)
	copy(dst[:], key)
}

type cryptoWriter struct {
	io.Writer
	cipher.AEAD
	noncer *noncer
	buf    []byte
}

// NewCryptoWriter wraps an io.Writer with AEAD encryption.
func NewCryptoWriter(w io.Writer, aead cipher.AEAD, seed32, read_pubkey *[32]byte) io.Writer {
	return newCryptoWriter(w, aead, seed32, read_pubkey)
}

func newCryptoWriter(w io.Writer, aead cipher.AEAD, seed32, read_pubkey *[32]byte) *cryptoWriter {
	return &cryptoWriter{
		Writer: w,
		AEAD:   aead,
		buf:    make([]byte, 2+aead.Overhead()+cryptoPayloadSizeMask+aead.Overhead()),
		noncer: NewFixedEndlessNoncer(seed32),
	}
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
func NewCryptoReader(r io.Reader, aead cipher.AEAD, seed32, read_scalar *[32]byte) io.Reader {
	return newCryptoReader(r, aead, seed32, read_scalar)
}

func newCryptoReader(r io.Reader, aead cipher.AEAD, seed32, read_scalar *[32]byte) *cryptoReader {
	return &cryptoReader{
		Reader: r,
		AEAD:   aead,
		buf:    make([]byte, cryptoPayloadSizeMask+aead.Overhead()),
		noncer: NewFixedEndlessNoncer(seed32),
	}
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
	num uint64
	// must be len of blake2b.Size256+4-1
	seed []byte
	// [0, blake2b.Size256)
	p int

	buf [chacha20poly1305.NonceSize]byte
}

func NewFixedEndlessNoncer(seed32 *[32]byte) *noncer {
	n := noncer{
		num:  0,
		seed: make([]byte, blake2b.Size256+4-1),
		p:    blake2b.Size256,
	}
	n.resetSeed(seed32[:])
	return &n
}

// Next must be used before the next call, since noncer reuse the result bytes
func (n *noncer) Next() []byte {
	if n.num == 0 {
		// next seed
		if n.p == blake2b.Size256 {
			// next sum
			seed32 := blake2b.Sum256(n.seed[:blake2b.Size256])
			n.resetSeed(seed32[:])
		}
		copy(n.buf[:4], n.seed[n.p:])
		n.p++
	}
	binary.BigEndian.PutUint64(n.buf[4:], n.num)
	n.num++
	return n.buf[:]
}

func (n *noncer) resetSeed(seed32 []byte) {
	n.p = 0
	copy(n.seed[:blake2b.Size256], seed32)
	copy(n.seed[blake2b.Size256:], n.seed[:4-1])
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
	Rand             io.Reader
	GetPrivateKey    func(serverPublic *[32]byte) (serverPrivate *[32]byte, err error)
	VerifyClient     func(serverPublic, clientPublic *[32]byte, auth []byte) (interface{}, error)
	TimestampValidIn uint64
}

func NewCryptoServerConn(c net.Conn, config *CryptoConnServerConfig) (net.Conn, interface{}, error) {
	if _, ok := c.(*cryptoStreamConn); ok {
		return c, nil, nil
	}

	if config.GetPrivateKey == nil {
		return nil, nil, errors.New("CryptoConnServerConfig: GetPrivateKey must be set")
	}

	randReader := config.Rand
	if randReader == nil {
		randReader = rand.Reader
	}

	var err error
	readScalar := func(s []byte) error {
		_, err = io.ReadFull(randReader, s)
		return err
	}

	var tmp_client_pubkey0, tmp_shared_key0, server_pubkey [32]byte

	//# ClientHello: length=292+n max_length=1024,max_n=732
	//version(2)
	//authorization_with_client_pubkey_length(2)
	//server_pubkey(32)
	//tmp_client_pubkey0(32)
	//nonce0(12)
	//chacha20poly1305(tmp_shared_key0, nonce0){
	//  nonce1(12)
	//  client_pubkey(32)
	//  authorization_with_client_pubkey(n)
	//  chacha20poly1305(argon2_shared_key, nonce1){
	//    blake2b_256_prev_bytes(32)
	//    timestamp(8)
	//    client_write_pubkey(32)
	//    client_read_pubkey(32)
	//    client_write_nonce_seed(16)
	//    client_read_nonce_seed(16)
	//  }
	//  poly1305(16)
	//}
	//poly1305(16)

	// 1. read frame
	_, err = io.ReadFull(c, tmp_client_pubkey0[:4])
	if err != nil {
		return nil, nil, err
	}
	if ver := binary.BigEndian.Uint16(tmp_client_pubkey0[:2]); ver != handshakeVersion {
		return nil, nil, fmt.Errorf("CryptoServerConn: only support ver %x, but got ver %x", handshakeVersion, ver)
	}

	n := binary.BigEndian.Uint16(tmp_client_pubkey0[2:4]) // authorization_with_client_pubkey_length
	buf := make([]byte, 292+n)                            // make the offset the same as client conn
	_, err = io.ReadFull(c, buf[4:])
	if err != nil {
		return nil, nil, err
	}
	copy(buf, tmp_client_pubkey0[:4])

	// 2. find main server_scalar
	copy(server_pubkey[:], buf[4:]) // server_pubkey
	server_scalar, err := config.GetPrivateKey(&server_pubkey)
	if err != nil {
		return nil, nil, err
	}

	// 3. open tmp chacha20poly1305
	// theirPublic
	copy(tmp_client_pubkey0[:], buf[36:]) // tmp_client_pubkey0
	curve25519.ScalarMult(&tmp_shared_key0, server_scalar, &tmp_client_pubkey0)
	tmp_aead0, err := chacha20poly1305.New(tmp_shared_key0[:])
	if err != nil {
		return nil, nil, err
	}

	//	tmp_aead0.Seal(buf[80:80], buf[68:80], buf[80:260+16+n], nil)
	_, err = tmp_aead0.Open(buf[80:80], buf[68:80], buf[80:], nil)
	if err != nil {
		return nil, nil, err
	}

	client_pubkey, shared_key := &tmp_client_pubkey0, &tmp_shared_key0
	copy(client_pubkey[:], buf[92:124])                                             // client_pubkey
	auth, err := config.VerifyClient(&server_pubkey, client_pubkey, buf[124:124+n]) // verify client_pubkey
	if err != nil {
		return nil, nil, err
	}

	// 4. open exchange chacha20poly1305
	salt := buf[68-8 : 92]
	curve25519ScalarMult(shared_key, server_scalar, client_pubkey, salt)
	exchange_aead0, err := chacha20poly1305.New(shared_key[:])
	if err != nil {
		return nil, nil, err
	}

	exchangeDataStart := 124 + n
	//  exchange_aead0.Seal(buf[exchangeDataStart:exchangeDataStart], buf[80:92], buf[exchangeDataStart:260+n], nil)
	clientHello, err := exchange_aead0.Open(buf[exchangeDataStart:exchangeDataStart], buf[80:92], buf[exchangeDataStart:260+16+n], nil)
	if err != nil {
		return nil, nil, err
	}

	//	clientHello now:
	//	  blake2b_256_prev_bytes(32)
	//    timestamp(8)
	//    client_write_pubkey(32)
	//    client_read_pubkey(32)
	//    client_write_nonce_seed(16)
	//    client_read_nonce_seed(16)

	//  blake2b_256_prev_bytes(32,156+n)
	//	copy(buf[124+n:], blake2b_256_prev_bytes[:])
	blake2b_256_prev_bytes := blake2b.Sum256(buf[:124+n])
	if !bytes.Equal(blake2b_256_prev_bytes[:], clientHello[:32]) {
		return nil, nil, fmt.Errorf("CryptoServerConn: blake2b_256_prev_bytes %X!=%X", blake2b_256_prev_bytes[:], clientHello[:32])
	}

	if binary.BigEndian.Uint64(clientHello[32:40]) > uint64(time.Now().Unix())+config.TimestampValidIn {
		return nil, nil, fmt.Errorf("CryptoConnConfig: ClientHello expired")
	}

	clientHello = clientHello[40:]
	//	clientHello now:
	//    client_write_pubkey(32)
	//    client_read_pubkey(32)
	//    client_write_nonce_seed(16)
	//    client_read_nonce_seed(16)

	//# ServerHello: length=192
	//tmp_server_pubkey0(32)
	//nonce2(12)
	//chacha20poly1305(tmp_shared_key1, nonce2){
	//  nonce3(12)
	//  chacha20poly1305(argon2_shared_key, nonce3){
	//    timestamp(8)
	//    server_read_pubkey(32)
	//    server_write_pubkey(32)
	//    server_read_nonce_seed(16)
	//    server_write_nonce_seed(16)
	//  }
	//  poly1305(16)
	//}
	//poly1305(16)

	// 5. build serverHello
	serverHello := buf[:192]
	tmp_server_pubkey0, tmp_server_scalar0 := &blake2b_256_prev_bytes, shared_key
	err = readScalar(tmp_server_scalar0[:])
	if err != nil {
		return nil, nil, err
	}
	curve25519.ScalarBaseMult(tmp_server_pubkey0, tmp_server_scalar0)
	copy(serverHello, tmp_server_pubkey0[:]) // tmp_server_pubkey0(32,32)
	tmp_server_shared_key0 := tmp_server_pubkey0
	curve25519.ScalarMult(tmp_server_shared_key0, tmp_server_scalar0, client_pubkey)
	tmp_aead1, err := chacha20poly1305.New(tmp_server_shared_key0[:])
	if err != nil {
		return nil, nil, err
	}
	_, err = io.ReadFull(randReader, serverHello[32:56]) // nonce2(12,44)+nonce3(12,56)
	if err != nil {
		return nil, nil, err
	}
	binary.BigEndian.PutUint64(serverHello[56:], uint64(time.Now().Unix())) // timestamp(8,64)
	server_read_pubkey, server_read_scalar := tmp_server_shared_key0, tmp_server_scalar0
	err = readScalar(server_read_scalar[:])
	if err != nil {
		return nil, nil, err
	}
	curve25519.ScalarBaseMult(server_read_pubkey, server_read_scalar)
	copy(serverHello[64:], server_read_pubkey[:]) // server_read_pubkey(32,96)
	server_read_shared_key := server_read_pubkey
	client_write_pubkey := server_scalar
	copy(client_write_pubkey[:], clientHello[:32])
	// keep server_read_scalar
	curve25519.ScalarMult(server_read_shared_key, server_read_scalar, client_write_pubkey)
	aead_c2s, err := chacha20poly1305.New(server_read_shared_key[:])
	if err != nil {
		return nil, nil, err
	}
	server_write_pubkey, server_write_scalar := server_read_pubkey, client_write_pubkey
	err = readScalar(server_write_scalar[:])
	if err != nil {
		return nil, nil, err
	}
	curve25519.ScalarBaseMult(server_write_pubkey, server_write_scalar)
	copy(serverHello[96:], server_read_pubkey[:]) // server_write_pubkey(32,128)
	server_write_shared_key := server_write_pubkey
	// keep client_read_pubkey
	client_read_pubkey := &server_pubkey
	copy(client_read_pubkey[:], clientHello[32:64])
	curve25519.ScalarMult(server_write_shared_key, server_write_scalar, client_read_pubkey)
	aead_s2c, err := chacha20poly1305.New(server_write_shared_key[:])
	if err != nil {
		return nil, nil, err
	}

	_, err = io.ReadFull(randReader, serverHello[128:160]) // server_read_write_nonce_seed(128,160)
	if err != nil {
		return nil, nil, err
	}
	//    client_write_nonce_seed(16) buf[228+n:244+n]
	//    client_read_nonce_seed(16) buf[244+n:260+n]
	//    server_read_nonce_seed(16) buf[128:144]
	//    server_write_nonce_seed(16) buf[144:160]

	c2sSeed := server_write_shared_key
	copy(c2sSeed[:16], buf[228+n:])
	copy(c2sSeed[16:], buf[128:])
	s2cSeed := server_write_scalar
	copy(s2cSeed[:16], buf[144:])
	copy(s2cSeed[16:], buf[244+n:])

	// 6. encrypt and send
	exchange_aead0.Seal(serverHello[56:56], serverHello[44:56], serverHello[56:160], nil)
	tmp_aead1.Seal(serverHello[44:44], serverHello[32:44], serverHello[44:176], nil)
	_, err = c.Write(serverHello)
	if err != nil {
		return nil, nil, err
	}

	// 7. create conn
	r := newCryptoReader(c, aead_c2s, c2sSeed, server_read_scalar)
	w := newCryptoWriter(c, aead_s2c, s2cSeed, client_read_pubkey)

	return &cryptoStreamConn{cryptoReader: r, cryptoWriter: w, c: c}, auth, nil
}

type CryptoConnConfig struct {
	Rand             io.Reader
	ServerPublic     *[32]byte
	ClientScalar     *[32]byte
	Authorization    []byte
	TimestampValidIn uint64
}

// NewCryptoConn wraps a stream-oriented net.Conn for client.
func NewCryptoConn(c net.Conn, config *CryptoConnConfig) (net.Conn, error) {
	if _, ok := c.(*cryptoStreamConn); ok {
		return c, nil
	}

	if config.ServerPublic == nil || config.ClientScalar == nil {
		return nil, errors.New("CryptoConnConfig: ServerPublic and ClientScalar must be set")
	}

	n := len(config.Authorization)
	if n > MaxAuthTokenSize {
		return nil, fmt.Errorf("CryptoConnConfig: Authorization size must smaller than %d, but got %d", MaxAuthTokenSize, n)
	}

	if config.TimestampValidIn == 0 {
		return nil, fmt.Errorf("CryptoConnConfig: TimestampValidIn must be set")
	}

	randReader := config.Rand
	if randReader == nil {
		randReader = rand.Reader
	}

	var err error
	readScalar := func(s []byte) error {
		_, err = io.ReadFull(randReader, s)
		return err
	}

	//# ClientHello: length=292+n max_length=1024,max_n=732
	//version(2)
	//authorization_with_client_pubkey_length(2)
	//server_pubkey(32)
	//tmp_client_pubkey0(32)
	//nonce0(12)
	//chacha20poly1305(tmp_shared_key0, nonce0){
	//  nonce1(12)
	//  client_pubkey(32)
	//  authorization_with_client_pubkey(n)
	//  chacha20poly1305(argon2_shared_key, nonce1){
	//    blake2b_256_prev_bytes(32)
	//    timestamp(8)
	//    client_write_pubkey(32)
	//    client_read_pubkey(32)
	//    client_write_nonce_seed(16) buf[228+n:244+n]
	//    client_read_nonce_seed(16) buf[244+n:260+n]
	//  }
	//  poly1305(16)
	//}
	//poly1305(16)

	var tmp_client_scalar0, tmp_client_pubkey0, tmp_shared_key0 [32]byte

	// 1. tmp_aead0 and exchange_aead0
	err = readScalar(tmp_client_scalar0[:])
	if err != nil {
		return nil, err
	}
	curve25519.ScalarBaseMult(&tmp_client_pubkey0, &tmp_client_scalar0)
	curve25519.ScalarMult(&tmp_shared_key0, &tmp_client_scalar0, config.ServerPublic)
	tmp_aead0, err := chacha20poly1305.New(tmp_shared_key0[:])
	if err != nil {
		return nil, err
	}

	// 2. ClientHello: length=292+n
	buf := make([]byte, 292+n)
	binary.BigEndian.PutUint16(buf, handshakeVersion) // version(2,2)
	binary.BigEndian.PutUint16(buf[2:], uint16(n))    // authorization_with_client_pubkey_length(2,4)
	copy(buf[4:], config.ServerPublic[:])             // server_pubkey(32,36)
	copy(buf[36:], tmp_client_pubkey0[:])             // tmp_client_pubkey0(32,68)
	_, err = io.ReadFull(randReader, buf[68:92])      // nonce0(12,80)+nonce1(12,92)
	if err != nil {
		return nil, err
	}

	salt := buf[68-8 : 92]
	curve25519ScalarMult(&tmp_shared_key0, config.ClientScalar, config.ServerPublic, salt)
	exchange_aead0, err := chacha20poly1305.New(tmp_shared_key0[:])
	if err != nil {
		return nil, err
	}

	client_pubkey := &tmp_shared_key0
	curve25519.ScalarBaseMult(client_pubkey, config.ClientScalar)
	copy(buf[92:], client_pubkey[:]) // client_pubkey(32,124)
	if n > 0 {
		copy(buf[124:], config.Authorization) // authorization_with_client_pubkey(n,124+n)
	}
	blake2b_256_prev_bytes := blake2b.Sum256(buf[:124+n])
	copy(buf[124+n:], blake2b_256_prev_bytes[:])                       // blake2b_256_prev_bytes(32,156+n)
	binary.BigEndian.PutUint64(buf[156+n:], uint64(time.Now().Unix())) // timestamp(8,164+n)

	client_write_scalar, client_write_pubkey := &tmp_client_scalar0, &tmp_client_pubkey0
	err = readScalar(client_write_scalar[:])
	if err != nil {
		return nil, err
	}
	curve25519.ScalarBaseMult(client_write_pubkey, client_write_scalar)
	copy(buf[164+n:], client_write_pubkey[:]) // client_write_pubkey(32,196+n)

	client_read_scalar, client_read_pubkey := &tmp_shared_key0, &blake2b_256_prev_bytes
	err = readScalar(client_read_scalar[:])
	if err != nil {
		return nil, err
	}
	curve25519.ScalarBaseMult(client_read_pubkey, client_read_scalar)
	copy(buf[196+n:], client_read_pubkey[:]) // client_read_pubkey(32,228+n)

	_, err = io.ReadFull(randReader, buf[228+n:260+n]) // client_write_read_nonce_seed(16+16,260+n)
	if err != nil {
		return nil, err
	}

	// 3.1 save client_write_read_nonce_seed before Seal
	var client_write_read_nonce_seed [32]byte
	copy(client_write_read_nonce_seed[:], buf[228+n:260+n])
	// 3.2 encrypt exchange data authorization_with_client_pubkey(n,124+n)
	exchangeDataStart := 124 + n
	exchange_aead0.Seal(buf[exchangeDataStart:exchangeDataStart], buf[80:92], buf[exchangeDataStart:260+n], nil)
	// 3.3 encrypt tmp data nonce0(12,80)+nonce1(12,92) start=80
	tmp_aead0.Seal(buf[80:80], buf[68:80], buf[80:260+16+n], nil)
	_, err = c.Write(buf)
	if err != nil {
		return nil, err
	}

	// 4. get server_hello
	serverHello := buf[:192]
	_, err = io.ReadFull(c, serverHello)
	if err != nil {
		return nil, err
	}

	//# ServerHello: length=192
	//tmp_server_pubkey0(32)
	//nonce2(12)
	//chacha20poly1305(tmp_shared_key1, nonce2){
	//  nonce3(12)
	//  chacha20poly1305(argon2_shared_key, nonce3){
	//    timestamp(8)
	//    server_read_pubkey(32)
	//    server_write_pubkey(32)
	//    server_read_nonce_seed(16) buf[128:144]
	//    server_write_nonce_seed(16) buf[144:160]
	//  }
	//  poly1305(16) buf[160:176]
	//}
	//poly1305(16)

	tmp_server_pubkey0, tmp_shared_key1 := client_write_pubkey, client_read_pubkey

	// 5. decrypt serverHello
	copy(tmp_server_pubkey0[:], serverHello)
	curve25519.ScalarMult(tmp_shared_key1, config.ClientScalar, tmp_server_pubkey0)
	tmp_aead1, err := chacha20poly1305.New(tmp_shared_key1[:])
	if err != nil {
		return nil, err
	}

	serverHello, err = tmp_aead1.Open(serverHello[44:44], serverHello[32:44], serverHello[44:], nil)
	if err != nil {
		return nil, err
	}

	serverHello, err = exchange_aead0.Open(serverHello[12:12], serverHello[:12], serverHello[12:], nil)
	if err != nil {
		return nil, err
	}

	// serverHello now:
	//    timestamp(8)
	//    server_read_pubkey(32)
	//    server_write_pubkey(32)
	//    server_read_nonce_seed(16)
	//    server_write_nonce_seed(16)

	if binary.BigEndian.Uint64(serverHello[:8]) > uint64(time.Now().Unix())+config.TimestampValidIn {
		return nil, fmt.Errorf("CryptoConnConfig: ServerHello expired")
	}

	// 6. session aead
	server_read_pubkey, server_read_shared_key := tmp_server_pubkey0, tmp_shared_key1
	copy(server_read_pubkey[:], serverHello[8:]) // server_read_pubkey(32,40)
	curve25519.ScalarMult(server_read_shared_key, client_write_scalar, server_read_pubkey)
	aead_c2s, err := chacha20poly1305.New(server_read_shared_key[:])
	if err != nil {
		return nil, err
	}

	server_write_pubkey, server_write_shared_key := client_write_scalar, server_read_shared_key
	copy(server_write_pubkey[:], serverHello[40:]) // server_write_pubkey(32,72)
	curve25519.ScalarMult(server_write_shared_key, client_read_scalar, server_write_pubkey)
	aead_s2c, err := chacha20poly1305.New(server_write_shared_key[:])
	if err != nil {
		return nil, err
	}

	// clientHello encrypted, so use
	// client_write_read_nonce_seed:
	//    client_write_nonce_seed(16) buf[228+n:244+n]
	//    client_read_nonce_seed(16) buf[244+n:260+n]
	//    server_read_nonce_seed(16) buf[128:144]
	//    server_write_nonce_seed(16) buf[144:160]

	// 7. create conn
	s2cSeed := client_write_scalar // server_write_nonce_seed+client_read_nonce_seed
	copy(s2cSeed[:16], buf[144:])
	copy(s2cSeed[16:], client_write_read_nonce_seed[16:])
	r := newCryptoReader(c, aead_s2c, s2cSeed, client_read_scalar)

	c2sSeed := &client_write_read_nonce_seed // client_write_nonce_seed+server_read_nonce_seed
	copy(c2sSeed[16:], buf[128:])
	w := newCryptoWriter(c, aead_c2s, c2sSeed, server_read_pubkey)

	return &cryptoStreamConn{cryptoReader: r, cryptoWriter: w, c: c}, nil
}
