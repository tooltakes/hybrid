package hybrid

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"golang.org/x/crypto/curve25519"
)

var (
	base64chars        = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	base64charsBlake2b = []byte("632b4264d09e9ed9764252cf1623f2f11cab4b4bf0aa66a79a5baca9d9586e1dcf538b737946cf426e3ecf711d3e9e239daecd31fcb717973b5f54054b156d1a")
)

func TestFixedEndlessNoncer(t *testing.T) {
	n, err := NewFixedEndlessNoncer(base64chars)
	if err != nil {
		t.Errorf("should no error, but got: %v", err)
		return
	}

	var nonce []byte
	if nonce = n.Next(); string(nonce) != "ABCDEFGHIJKL" {
		t.Errorf("should get ABCDEFGHIJKL, but got %s", nonce)
		return
	}

	n.p = 63
	if nonce = n.Next(); string(nonce) != "/ABCDEFGHIJK" {
		t.Errorf("should get /ABCDEFGHIJK, but got %s", nonce)
		return
	}

	var noncestr string
	if noncestr = hex.EncodeToString(n.Next()); noncestr != "632b4264d09e9ed9764252cf" {
		t.Errorf("should get 632b4264d09e9ed9764252cf, but got %s", noncestr)
		return
	}
}

func TestCryptoConnHandshake(t *testing.T) {
	var serverScalar, serverPub [32]byte
	_, err := rand.Read(serverScalar[:])
	if err != nil {
		t.Errorf("should no error, but got: %v", err)
		return
	}
	curve25519.ScalarBaseMult(&serverPub, &serverScalar)

	var clientScalar, clientPub [32]byte
	_, err = rand.Read(clientScalar[:])
	if err != nil {
		t.Errorf("should no error, but got: %v", err)
		return
	}
	curve25519.ScalarBaseMult(&clientPub, &clientScalar)

	server, client := net.Pipe()
	errCh := make(chan error)
	go func() {
		defer server.Close()
		serverConfig := &CryptoConnServerConfig{
			GetPrivateKey: func(serverPublic, clientPublic []byte) (serverPrivate *[32]byte, err error) {
				if bytes.Compare(clientPublic, clientPub[:])|bytes.Compare(serverPublic, serverPub[:]) == 0 {
					return &serverScalar, nil
				}
				return nil, errors.New("serverPrivate not found")
			},
		}
		serverConn, err := NewCryptoServerConn(&copyWriteConn{server}, serverConfig)
		if err != nil {
			errCh <- err
			return
		}
		errCh <- nil
		serverConn.Write([]byte("987654321"))

		b := make([]byte, 10)
		n, err := serverConn.Read(b)
		if err != nil && err != io.EOF {
			fmt.Println("server err:", err)
			errCh <- err
			return
		}
		if string(b[:n]) != "9876543210" {
			fmt.Println("server err:", string(b[:n]))
			errCh <- fmt.Errorf("server should get 9876543210, but got %s", b[:n])
			return
		}
		errCh <- nil
	}()

	clientConfig := &CryptoConnConfig{
		ServerPublic: &serverPub,
		ClientScalar: &clientScalar,
	}
	clientConn, err := NewCryptoConn(&copyWriteConn{client}, clientConfig)
	if err != nil {
		t.Errorf("client should no error, but got: %v", err)
	}

	err = <-errCh
	if err != nil {
		t.Errorf("server write should no error, but got: %v", err)
		return
	}

	b := make([]byte, 9)
	n, err := clientConn.Read(b)
	if err != nil && err != io.EOF {
		t.Errorf("client read should no error, but got: %v", err)
		return
	}

	if string(b[:n]) != "987654321" {
		t.Errorf("client should get 987654321, but got ", string(b))
		return
	}

	_, err = clientConn.Write([]byte("9876543210"))
	if err != nil {
		t.Errorf("client write should no error, but got: %v", err)
		return
	}

	err = <-errCh
	if err != nil {
		t.Errorf("server read should no error, but got: %v", err)
		return
	}
}

type copyWriteConn struct {
	net.Conn
}

func (c *copyWriteConn) Write(b []byte) (int, error) {
	p := make([]byte, len(b))
	copy(p, b)
	return c.Conn.Write(p)
}
