package hybridipfs

import (
	"context"
	"net"
	"sync"

	inet "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-net"
	p2pnet "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-net"
	ipfspeer "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peer"
	pstore "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peerstore"
	pro "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-protocol"
)

type streamAddr struct {
	protocol string
	peer     string
}

func (sa *streamAddr) Network() string { return sa.protocol }
func (sa *streamAddr) String() string  { return sa.peer }

type stdStream struct {
	p2pnet.Stream
}

func (st *stdStream) LocalAddr() net.Addr {
	return &streamAddr{
		protocol: string(st.Protocol()),
		peer:     st.Stream.Conn().LocalPeer().Pretty(),
	}
}

func (st *stdStream) RemoteAddr() net.Addr {
	return &streamAddr{
		protocol: string(st.Protocol()),
		peer:     st.Stream.Conn().RemotePeer().Pretty(),
	}
}

func (hi *Ipfs) Dial(peerID ipfspeer.ID, protocol string) (net.Conn, error) {
	ipfsNode, err := hi.getDaemonNode()
	if err != nil {
		return nil, err
	}

	peerInfo := pstore.PeerInfo{ID: peerID}
	err = ipfsNode.PeerHost.Connect(hi.ctx, peerInfo)
	if err != nil {
		return nil, err
	}

	protoId := pro.ID(protocol)
	stream, err := ipfsNode.PeerHost.NewStream(hi.ctx, peerID, protoId)
	if err != nil {
		return nil, err
	}

	return &stdStream{Stream: stream}, nil
}

func (hi *Ipfs) Listen(protocol string, match func(string) bool) (*Listener, error) {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	// add to listeners
	ln, ok := hi.listeners[protocol]
	if ok {
		return nil, ErrProtocolListened
	}

	ctx, cancel := context.WithCancel(hi.ctx)
	ln = &Listener{
		hi:       hi,
		protocol: protocol,
		match:    match,
		self:     hi.ipfsNode.Identity.String(),
		conCh:    make(chan p2pnet.Stream),
		ctx:      ctx,
		cancel:   cancel,
	}
	hi.listeners[protocol] = ln

	if hi.isOnline() {
		hi.setStreamHandlerToDaemonHost(ln)
	}
	return ln, nil
}

func (hi *Ipfs) setStreamHandlerToDaemonHost(ln *Listener) {
	protoId := pro.ID(ln.protocol)
	if ln.match == nil {
		hi.ipfsNode.PeerHost.SetStreamHandler(protoId, ln.onStream)
	} else {
		hi.ipfsNode.PeerHost.SetStreamHandlerMatch(protoId, ln.match, ln.onStream)
	}
}

type StreamVerify func(is inet.Stream) bool

type Listener struct {
	hi       *Ipfs
	self     string
	protocol string
	match    func(string) bool
	verify   StreamVerify

	conCh     chan p2pnet.Stream
	ctx       context.Context
	cancel    func()
	closeOnce sync.Once
}

// SetVerify cannot be used when Accepting
func (ln *Listener) SetVerify(verify StreamVerify) {
	ln.verify = verify
}

func (ln *Listener) onStream(stream p2pnet.Stream) {
	select {
	case ln.conCh <- stream:
	case <-ln.ctx.Done():
		stream.Close()
	}
}

func (lst *Listener) Accept() (net.Conn, error) {
	for {
		select {
		case stream := <-lst.conCh:
			if lst.verify == nil || lst.verify(stream) {
				return &stdStream{Stream: stream}, nil
			}
			stream.Close()
		case <-lst.ctx.Done():
			return nil, nil
		}
	}
}

func (lst *Listener) Addr() net.Addr {
	return &streamAddr{
		protocol: lst.protocol,
		peer:     lst.self,
	}
}

func (lst *Listener) Close() error {
	lst.closeOnce.Do(lst.close)
	return nil
}

func (lst *Listener) close() {
	hi := lst.hi

	hi.mu.Lock()
	delete(hi.listeners, lst.protocol)
	hi.mu.Unlock()

	lst.cancel()
	lst.hi = nil
}

func (lst *Listener) Protocol() string         { return lst.protocol }
func (lst *Listener) Context() context.Context { return lst.ctx }
