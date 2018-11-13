package ipfs

import (
	"github.com/ipsn/go-ipfs/core/corerepo"

	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	mh "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multihash"
)

func (hi *Ipfs) GC() ([]mh.Multihash, error) {
	gcOutChan := corerepo.GarbageCollectAsync(hi.getAnyNode(), hi.ctx)
	killed := make([]mh.Multihash, 0, 8)

	// CollectResult blocks until garbarge collection is finished:
	err := corerepo.CollectResult(hi.ctx, gcOutChan, func(k cid.Cid) {
		killed = append(killed, k.Hash())
	})

	if err != nil {
		return nil, err
	}

	return killed, nil
}
