package hybridipfs

import (
	"io"

	coreunix "github.com/ipsn/go-ipfs/core/coreunix"

	uio "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/io"
)

func (nd *Node) Cat(b58 string) (uio.DagReader, error) {
	return coreunix.Cat(nd.ctx, nd.ipfsNode, b58)
}

func (nd *Node) Add(r io.Reader) (string, error) {
	return coreunix.Add(nd.ipfsNode, r)
}
