package hybridipfs

import "github.com/ipsn/go-ipfs/core"

func (nd *Node) getDaemonNode() (*core.IpfsNode, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	if !nd.isOnline() {
		return nil, ErrIsOffline
	}
	return nd.ipfsNode, nil
}

func (nd *Node) getOfflineNode() (*core.IpfsNode, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	if nd.isOnline() {
		return nil, ErrIsOnline
	}
	return nd.ipfsNode, nil
}

func (nd *Node) getAnyNode() *core.IpfsNode {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.ipfsNode, nil
}
