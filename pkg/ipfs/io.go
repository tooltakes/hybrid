package ipfs

import (
	coreiface "github.com/ipsn/go-ipfs/core/coreapi/interface"
	"github.com/ipsn/go-ipfs/core/coreapi/interface/options"

	files "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-files"
)

func (hi *Ipfs) Get(path string) (coreiface.UnixfsFile, error) {
	p, err := coreiface.ParsePath(path)
	if err != nil {
		return nil, err
	}

	api, err := hi.coreAPI()
	if err != nil {
		return nil, err
	}

	return api.Unixfs().Get(hi.ctx, p)
}

func (hi *Ipfs) Add(file files.File, settings options.UnixfsAddSettings) (coreiface.ResolvedPath, error) {
	api, err := hi.coreAPI()
	if err != nil {
		return nil, err
	}

	return api.Unixfs().Add(hi.ctx, file, func(target *options.UnixfsAddSettings) error {
		if settings.CidVersion == 0 {
			settings.CidVersion = target.CidVersion
		}
		*target = settings
		return nil
	})
}
