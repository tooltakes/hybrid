package hybridipfs

import (
	"fmt"

	"github.com/ipsn/go-ipfs/core"

	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-path"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-path/resolver"
	uio "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/io"
	mh "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multihash"
)

func (hi *Ipfs) Pin(hash string) error {
	ipfsNode, err := hi.getDaemonNode()
	if err != nil {
		return err
	}

	rslv := &resolver.Resolver{
		DAG:         ipfsNode.DAG,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	p, err := path.ParsePath(hash)
	if err != nil {
		return err
	}

	// Lock the store:
	defer ipfsNode.Blockstore.PinLock().Unlock()

	dagnode, err := core.Resolve(hi.ctx, ipfsNode.Namesys, rslv, p)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	err = ipfsNode.Pinning.Pin(hi.ctx, dagnode, true)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	return ipfsNode.Pinning.Flush()
}

func (hi *Ipfs) Unpin(hash string) error {
	ipfsNode, err := hi.getDaemonNode()
	if err != nil {
		return err
	}

	mhash, err := mh.FromB58String(hash)
	if err != nil {
		return err
	}

	// Lock the store:
	defer ipfsNode.Blockstore.PinLock().Unlock()

	cid := cid.NewCidV0(mhash)
	err = ipfsNode.Pinning.Unpin(hi.ctx, cid, true)
	if err != nil {
		return err
	}

	return ipfsNode.Pinning.Flush()
}

func (hi *Ipfs) IsPinned(hash string) (bool, error) {
	ipfsNode, err := hi.getDaemonNode()
	if err != nil {
		return false, err
	}

	mhash, err := mh.FromB58String(hash)
	if err != nil {
		return false, err
	}

	cid := cid.NewCidV0(mhash)
	mode, _, err := ipfsNode.Pinning.IsPinned(cid)
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
