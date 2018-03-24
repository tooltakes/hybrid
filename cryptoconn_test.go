package hybrid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"testing"

	"golang.org/x/crypto/curve25519"
)

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
			GetPrivateKey: func(serverPublic *[32]byte) (serverPrivate *[32]byte, err error) {
				if *serverPublic == serverPub {
					clone := serverScalar
					return &clone, nil
				}
				return nil, fmt.Errorf("GetPrivateKey: %X != %X", *serverPublic, serverPub)
			},
			VerifyClient: func(serverPublic, clientPublic *[32]byte, auth []byte) (interface{}, error) {
				if *clientPublic == clientPub {
					return nil, nil
				}
				return nil, fmt.Errorf("VerifyClient: %X != %X", *clientPublic, clientPub)
			},
			TimestampValidIn: 5,
		}
		serverConn, _, err := NewCryptoServerConn(&copyWriteConn{server}, serverConfig)
		if err != nil {
			panic(err)
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
		ServerPublic:     &serverPub,
		ClientScalar:     &clientScalar,
		Authorization:    []byte("I am valid"),
		TimestampValidIn: 5,
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
		t.Errorf("client should get 987654321, but got %s", string(b))
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

func hexOf(d []byte) string { return hex.EncodeToString(d) }
