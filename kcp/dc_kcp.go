package hybridkcp

import (
	"net"

	"github.com/xtaci/kcp-go"
)

// Creator

type KCPListenerCreator struct {
	block kcp.BlockCrypt
	addr  string
}

func NewKCPListenerCreator(block kcp.BlockCrypt, addr string) *KCPListenerCreator {
	return &KCPListenerCreator{
		block: block,
		addr:  addr,
	}
}

func (c *KCPListenerCreator) Create() (net.Listener, error) {
	return kcp.ListenWithOptions(c.addr, c.block, 0, 0)
}

// Dialer

type KCPDialer struct {
	Block       kcp.BlockCrypt
	ServerNames []string
}

func (d *KCPDialer) Dial(addr string) (net.Conn, []string, error) {
	c, err := kcp.DialWithOptions(addr, d.Block, 0, 0)
	return c, d.ServerNames, err
}
