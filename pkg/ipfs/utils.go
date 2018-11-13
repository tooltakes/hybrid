package ipfs

import (
	"github.com/ipsn/go-ipfs/core"
	coreiface "github.com/ipsn/go-ipfs/core/coreapi/interface"
)

func (hi *Ipfs) getDaemonNode() (*core.IpfsNode, error) {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	if !hi.isOnline() {
		return nil, ErrIsOffline
	}
	return hi.ipfsNode, nil
}

func (hi *Ipfs) getOfflineNode() (*core.IpfsNode, error) {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	if hi.isOnline() {
		return nil, ErrIsOnline
	}
	return hi.ipfsNode, nil
}

func (hi *Ipfs) getAnyNode() *core.IpfsNode {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	return hi.ipfsNode
}

func (hi *Ipfs) coreAPI() (coreiface.CoreAPI, error) {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	if !hi.isOnline() {
		return nil, ErrIsOffline
	}
	return hi.api, nil
}
