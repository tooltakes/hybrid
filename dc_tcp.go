package hybrid

import (
	"net"
)

// Creator

type TCPListenerCreator struct {
	config *CryptoConnServerConfig
	addr   string
}

func NewTCPListenerCreator(addr string, config *CryptoConnServerConfig) *TCPListenerCreator {
	return &TCPListenerCreator{
		addr:   addr,
		config: config,
	}
}

func (c *TCPListenerCreator) Create() (l net.Listener, err error) {
	l, err = net.Listen("tcp", c.addr)
	if err != nil {
		return
	}
	if c.config != nil {
		l = &CryptoListener{
			Listener: l,
			Config:   c.config,
		}
	}
	return
}

// Dialer

type TCPDialer struct {
	Config      *CryptoConnConfig
	ServerNames []string
}

func (d *TCPDialer) Dial(addr string) (net.Conn, []string, error) {
	c, err := net.Dial("tcp", addr)
	if err == nil && d.Config != nil {
		conn := c
		c, err = NewCryptoConn(c, d.Config)
		if err != nil {
			err = &net.OpError{
				Op:     "handshake",
				Net:    "crypto",
				Source: conn.RemoteAddr(),
				Addr:   conn.LocalAddr(),
				Err:    &HandshakeError{err},
			}
		}
	}
	return c, d.ServerNames, err
}
