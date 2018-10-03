package hybridipfs

import (
	"fmt"
	"runtime"

	fsrepo "github.com/ipsn/go-ipfs/repo/fsrepo"

	id "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p/p2p/protocol/identify"
)

type Version struct {
	Ipfs     string
	LibP2P   string
	GoLibP2P string
	Repo     string
	Golang   string
	System   string
}

var version *Version

func GetVersion() *Version {
	if version == nil {
		version = &Version{
			Ipfs:     fmt.Sprintf("%s-%s", version.CurrentVersionNumber, version.CurrentCommit),
			LibP2P:   id.LibP2PVersion,
			GoLibP2P: id.ClientVersion,
			Repo:     fsrepo.RepoVersion,
			Golang:   runtime.Version(),
			System:   fmt.Sprintf("%s/%s", runtime.GOARCH, runtime.GOOS),
		}
	}
	return version
}
