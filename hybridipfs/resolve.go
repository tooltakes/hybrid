package hybridipfs

import (
	"context"

	blocks "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-block-format"
	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	pstore "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peerstore"
	mh "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multihash"
)

// addBlock creates a new block with `data`.
// The hash of the data is returned.
// It is no error if the block already exists.
func (nd *Node) addBlock(data []byte) (mh.Multihash, error) {
	block := blocks.NewBlock(data)
	if err := nd.getAnyNode().Blocks.AddBlock(block); err != nil {
		return nil, err
	}

	return block.Cid().Hash(), nil
}

func (nd *Node) PublishName(name string) error {
	// Build all names under we can find this node:
	fullName := "brig:" + string(name)
	hash, err := nd.addBlock([]byte(fullName))
	if err != nil {
		return err
	}

	return nil
}

// Locate finds the object pointed to by `hash`. it will wait
// for max `timeout` duration if it got less than `n` items in that time.
// if `n` is less than 0, all reachable peers that have `hash` will be returned.
// if `n` is 0, locate will return immeditately.
// this operation requires online-mode.
func (nd *Node) ResolveName(ctx context.Context, name string) ([]pstore.PeerInfo, error) {
	name = "brig:" + name
	hash := blocks.NewBlock([]byte(name)).Multihash()

	k, err := cid.Decode(hash.B58String())
	if err != nil {
		return nil, err
	}

	ipfsNode, err := nd.getDaemonNode()
	if err != nil {
		return nil, err
	}

	peers := ipfsNode.Routing.FindProvidersAsync(ctx, k, 10)
	infos := make([]pstore.PeerInfo)
	for info := range peers {
		infos = append(infos, info)
	}
	return infos, nil
}
