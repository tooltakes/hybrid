package ipfs

import (
	"context"
	"net/http"
	"os"

	"github.com/empirefox/hybrid/pkg/netutil"
	oldcmds "github.com/ipsn/go-ipfs/commands"
	"github.com/ipsn/go-ipfs/core"
	coreiface "github.com/ipsn/go-ipfs/core/coreapi/interface"
	"github.com/ipsn/go-ipfs/repo"
	"github.com/ipsn/go-ipfs/repo/fsrepo"
	migrate "github.com/ipsn/go-ipfs/repo/fsrepo/migrations"
)

// MigrateStandalone require hack on:
// github.com/ipsn/go-ipfs/repo/fsrepo/migrations/migrations.go
func MigrateStandalone(ctx context.Context, config *Config, to int, tempRepo string) error {
	m, err := NewTempNode(ctx, tempRepo, config)
	if err != nil {
		return err
	}
	defer m.Close()
	return m.RunMigration(to)
}

type TempNode struct {
	ctx    context.Context
	cancel context.CancelFunc
	config Config
	api    coreiface.CoreAPI
	rt     http.RoundTripper
}

func NewTempNode(ctx context.Context, repoPath string, config *Config) (*TempNode, error) {
	ctx, cancel := context.WithCancel(ctx)
	m := TempNode{
		ctx:    ctx,
		cancel: cancel,
		config: *config,
	}
	m.config.RepoPath = repoPath
	r, err := m.openTempRepo()
	if err != nil { // repo is owned by the node
		cancel()
		return nil, err
	}

	nd, err := core.NewNode(m.ctx, &core.BuildCfg{
		Repo: r,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	cctx := &oldcmds.Context{
		ConfigRoot:    m.config.RepoPath,
		LoadConfig:    fsrepo.ConfigAt,
		ReqLog:        &oldcmds.ReqLog{},
		ConstructNode: func() (*core.IpfsNode, error) { return nd, nil },
	}

	api, err := cctx.GetApi()
	if err != nil {
		cancel()
		return nil, err
	}
	m.api = api

	handler, err := newGatewayHandler(nd, cctx, &m.config)
	if err != nil {
		cancel()
		return nil, err
	}
	m.rt = netutil.NewHandlerTransport(handler)

	return &m, nil
}

func (m *TempNode) RunMigration(newv int) error {
	migrate.SetTransport(m)
	return migrate.RunMigration(newv)
}

func (m *TempNode) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case <-m.ctx.Done():
		return nil, http.ErrSkipAltProtocol
	}
	if req.URL.Scheme == "https" {
		req.URL.Scheme = "http"
	}
	return m.rt.RoundTrip(req)
}

func (m *TempNode) Get(path string) (coreiface.UnixfsFile, error) {
	p, err := coreiface.ParsePath(path)
	if err != nil {
		return nil, err
	}
	return m.api.Unixfs().Get(m.ctx, p)
}

func (m *TempNode) Close() error {
	m.cancel()
	return nil
}

func (m *TempNode) openTempRepo() (repo.Repo, error) {
	r, err := fsrepo.Open(m.config.RepoPath)
	if err == nil {
		return r, nil
	}

	if err != fsrepo.ErrNeedMigration {
		return nil, err
	}

	err = os.RemoveAll(m.config.RepoPath)
	if err != nil {
		return nil, err
	}

	err = InitWithDefaultsIfNotExist(m.config.RepoPath, nil)
	if err != nil {
		return nil, err
	}

	return fsrepo.Open(m.config.RepoPath)
}
