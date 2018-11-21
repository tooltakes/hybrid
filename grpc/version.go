package grpc

import (
	"fmt"
	"runtime"

	"github.com/empirefox/hybrid/pkg/ipfsdial"
	ipfs "github.com/ipfs/go-ipfs"
	"github.com/ipsn/go-ipfs/repo/fsrepo"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p/p2p/protocol/identify"
)

func GetVersion() *Version {
	return &Version{
		DialIpfsProtocol: ipfsdial.CurrentProtocol,
		Ipfs:             fmt.Sprintf("%s-%s", ipfs.CurrentVersionNumber, ipfs.CurrentCommit),
		IpfsRepo:         int32(fsrepo.RepoVersion),
		LibP2PProtocol:   identify.LibP2PVersion,
		GoLibP2P:         identify.ClientVersion,
		Golang:           runtime.Version(),
		System:           fmt.Sprintf("%s/%s", runtime.GOARCH, runtime.GOOS),
	}
}
