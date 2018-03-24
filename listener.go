package hybrid

import (
	"net"

	"go.uber.org/zap"
)

type HandshakeError struct {
	error
}

func NewHandshakeError(err error) *HandshakeError {
	return &HandshakeError{err}
}

func (e *HandshakeError) Temporary() bool { return true }

type CryptoListener struct {
	Log *zap.Logger
	net.Listener
	CryptoConnServerConfig
}

func (ln *CryptoListener) Accept() (net.Conn, error) {
	c, err := ln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	cn, _, err := NewCryptoServerConn(c, &ln.CryptoConnServerConfig)
	if err != nil {
		ln.Log.Error("NewCryptoServerConn", zap.Error(err))
		err = &net.OpError{
			Op:     "handshake",
			Net:    "x25519",
			Source: c.RemoteAddr(),
			Addr:   c.LocalAddr(),
			Err:    NewHandshakeError(err),
		}
		c.Close()
	}
	return cn, err
}
