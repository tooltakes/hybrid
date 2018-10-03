package hybridipfs

import (
	"fmt"

	core "github.com/ipsn/go-ipfs/core"
	"github.com/ipsn/go-ipfs/path"
	"github.com/ipsn/go-ipfs/path/resolver"
	uio "github.com/ipsn/go-ipfs/unixfs/io"

	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	mh "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multihash"
)

func (nd *Node) Pin(hash string) error {
	// Lock the store:
	defer nd.ipfsNode.Blockstore.PinLock().Unlock()

	rslv := &resolver.Resolver{
		DAG:         nd.ipfsNode.DAG,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	p, err := path.ParsePath(hash)
	if err != nil {
		return err
	}

	dagnode, err := core.Resolve(nd.ctx, nd.ipfsNode.Namesys, rslv, p)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	err = nd.ipfsNode.Pinning.Pin(nd.ctx, dagnode, true)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	return nd.ipfsNode.Pinning.Flush()
}

func (nd *Node) Unpin(hash string) error {
	// Lock the store:
	defer nd.ipfsNode.Blockstore.PinLock().Unlock()

	mhash, err := mh.FromB58String(hash)
	if err != nil {
		return err
	}

	cid := cid.NewCidV0(mhash)
	err = nd.ipfsNode.Pinning.Unpin(nd.ctx, cid, true)
	if err != nil {
		return err
	}

	return nd.ipfsNode.Pinning.Flush()
}

func (nd *Node) IsPinned(hash string) (bool, error) {
	mhash, err := mh.FromB58String(hash)
	if err != nil {
		return false, err
	}

	cid := cid.NewCidV0(mhash)
	mode, _, err := nd.ipfsNode.Pinning.IsPinned(cid)
	if err != nil {
		return false, err
	}

	switch mode {
	case "direct", "internal":
		return true, nil
	case "indirect", "recursive":
		return true, nil
	default:
		return false, nil
	}
}
