package hybrid

import "net"

type HandshakeError struct {
	error
}

func (e *HandshakeError) Temporary() bool { return true }

type CryptoListener struct {
	net.Listener
	Config *CryptoConnServerConfig
}

func (l *CryptoListener) Accept() (c net.Conn, err error) {
	c, err = l.Listener.Accept()
	if err != nil {
		return
	}
	conn := c
	c, err = NewCryptoServerConn(c, l.Config)
	if err != nil {
		err = &net.OpError{
			Op:     "handshake",
			Net:    "crypto",
			Source: conn.RemoteAddr(),
			Addr:   conn.LocalAddr(),
			Err:    &HandshakeError{err},
		}
	}
	return
}
