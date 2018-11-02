package hybridipfs

import (
	"fmt"
	"runtime"

	ipfs "github.com/ipfs/go-ipfs"
	"github.com/ipsn/go-ipfs/repo/fsrepo"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p/p2p/protocol/identify"
)

type Version struct {
	Ipfs     string
	LibP2P   string
	GoLibP2P string
	Repo     int
	Golang   string
	System   string
}

var version *Version

func GetVersion() *Version {
	if version == nil {
		version = &Version{
			Ipfs:     fmt.Sprintf("%s-%s", ipfs.CurrentVersionNumber, ipfs.CurrentCommit),
			LibP2P:   identify.LibP2PVersion,
			GoLibP2P: identify.ClientVersion,
			Repo:     fsrepo.RepoVersion,
			Golang:   runtime.Version(),
			System:   fmt.Sprintf("%s/%s", runtime.GOARCH, runtime.GOOS),
		}
	}
	return version
}
