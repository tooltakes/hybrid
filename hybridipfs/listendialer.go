package hybridipfs

import (
	"context"
	"errors"
	"net"
	"sync"

	ma "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr-net"
)

var (
	errListenDialerClosed = errors.New("ListenDialer is closed")
)

type Dialer interface {
	Dial(ctx context.Context) (manet.Conn, error)
	Multiaddr() ma.Multiaddr
	Addr() net.Addr
}

func NewListenDialer(listenerAddr, dialerAddr ma.Multiaddr) (manet.Listener, Dialer) {
	dl := &dialListener{
		combinedAddr: combinedAddr{listenerAddr},
		connCh:       make(chan manet.Conn),
		doneCh:       make(chan struct{}),
	}
	return dl, dialer{
		combinedAddr: combinedAddr{dialerAddr},
		dialListener: dl,
	}
}

type dialListener struct {
	combinedAddr
	connCh    chan manet.Conn
	doneCh    chan struct{}
	closeOnce sync.Once
}

// Accept waits for and returns the next connection to the listener.
func (dl *dialListener) Accept() (manet.Conn, error) {
	select {
	case c := <-dl.connCh:
		return c, nil
	case <-dl.doneCh:
		return nil, errListenDialerClosed
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (dl *dialListener) Close() error {
	dl.closeOnce.Do(func() { close(dl.doneCh) })
	return nil
}

type conn struct {
	net.Conn
	laddr ma.Multiaddr
	raddr ma.Multiaddr
}

// LocalMultiaddr returns the local address associated with
// this connection
func (c *conn) LocalMultiaddr() ma.Multiaddr {
	return c.laddr
}

// RemoteMultiaddr returns the remote address associated with
// this connection
func (c *conn) RemoteMultiaddr() ma.Multiaddr {
	return c.raddr
}

type dialer struct {
	combinedAddr
	*dialListener
}

func (d dialer) Dial(ctx context.Context) (manet.Conn, error) {
	nclient, nserver := net.Pipe()
	client := &conn{nclient, d.maddr, d.dialListener.maddr}
	server := &conn{nserver, d.dialListener.maddr, d.maddr}
	select {
	case d.connCh <- server:
		return client, nil
	case <-d.doneCh:
		nclient.Close()
		return nil, errListenDialerClosed
	case <-ctx.Done():
		nclient.Close()
		return nil, ctx.Err()
	}
}

type combinedAddr struct {
	maddr ma.Multiaddr
}

func (ca combinedAddr) Multiaddr() ma.Multiaddr {
	return ca.maddr
}
func (ca combinedAddr) Addr() net.Addr {
	addr := netAddr(ca.maddr.String())
	return &addr
}

type netAddr string

func (a *netAddr) Network() string { return "local" }
func (a *netAddr) String() string  { return string(*a) }
