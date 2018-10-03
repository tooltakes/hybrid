package hybridipfs

import (
	corerepo "github.com/ipsn/go-ipfs/core/corerepo"

	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	mh "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multihash"
)

func (nd *Node) GC() ([]mh.Multihash, error) {
	gcOutChan := corerepo.GarbageCollectAsync(nd.ipfsNode, nd.ctx)
	killed := make([]mh.Multihash)

	// CollectResult blocks until garbarge collection is finished:
	err := corerepo.CollectResult(nd.ctx, gcOutChan, func(k *cid.Cid) {
		killed = append(killed, k.Hash())
	})

	if err != nil {
		return nil, err
	}

	return killed, nil
}
