package grpc

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/dgraph-io/badger"
	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/node"
	"github.com/empirefox/hybrid/pkg/authstore"
	"github.com/empirefox/hybrid/pkg/badgerutil"
	"github.com/empirefox/hybrid/pkg/bufpool"
	"github.com/empirefox/hybrid/pkg/ipfs"
	multierror "github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	ipfsconfig "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-config"
)

var (
	StorePrefixVerifyKey = []byte("v/")
	StorePrefixSignKey   = []byte("s/")
)

type Service struct {
	log            *zap.Logger
	config         *config.Config
	node           *node.Node
	ipfs           *ipfs.Ipfs
	db             *badger.DB
	verifyKeystore *authstore.KeyStore
	cancel         context.CancelFunc

	stoppedErr error
	stopped    chan struct{}
}

func Start(ctx context.Context, root string, configBindId uint32) (*Service, error) {
	// 1. load config, root can be empty
	c, err := config.LoadConfig(&config.Config{RootPath: root})
	if err != nil {
		return nil, fmt.Errorf("LoadConfig err: %v", err)
	}

	// 2. log
	log, err := NewLogger(c)
	if err != nil {
		return nil, fmt.Errorf("NewLogger err: %v", err)
	}

	s := Service{log: log}
	defer func() {
		if s.node == nil {
			log.Sync()
		}
	}()

	// 3. migrate-standalone need set IPFS_PATH
	t, err := c.ConfigTree()
	if err != nil {
		log.Error("ConfigTree", zap.Error(err))
		return nil, err
	}
	os.Setenv(ipfsconfig.EnvDir, t.IpfsPath)

	// 4. k/v db
	db, err := badgerutil.NewBadger(t.StorePath, nil)
	if err != nil {
		log.Error("NewBadger", zap.Error(err))
		return nil, err
	}
	defer func() {
		if s.node == nil {
			db.Close()
		}
	}()

	// 5. verifier
	verifyKeystore, err := authstore.New(&authstore.Config{
		DB:         db,
		Prefix:     StorePrefixVerifyKey,
		BufferPool: bufpool.Default1K,
	})
	if err != nil {
		log.Error("New vierify keystore", zap.Error(err))
		return nil, err
	}
	// TODO support more verify? or support load from ~/.ssh/?
	verifier := NewVerifier(verifyKeystore, log)

	// 6. create ipfs
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer func() {
		if s.node == nil {
			cancel()
		}
	}()
	hi, err := node.NewIpfs(ctx, c, log)
	if err != nil {
		log.Error("NewIpfs", zap.Error(err))
		return nil, err
	}

	// 7. create and start hybrid node
	node, err := node.New(node.Config{
		Log:          log,
		Config:       c,
		Ipfs:         hi,
		Verify:       verifier.HybridVerify,
		LocalServers: map[string]http.Handler{},
		ConfigBindId: configBindId,
	})

	if err != nil {
		log.Error("NewClient", zap.Error(err))
		return nil, err
	}
	defer func() {
		if s.node == nil {
			node.Close()
		}
	}()

	// 8. ipfs online
	err = hi.Connect()
	if err != nil {
		log.Error("ipfs.Connect", zap.Error(err))
		return nil, err
	}

	go func(s *Service, ctx context.Context) {
		select {
		case <-ctx.Done():
		}
		s.node.Close()
		s.db.Close()
		s.waitUntilStopped()
	}(&s, ctx)

	s.config = c
	s.node = node
	s.ipfs = hi
	s.db = db
	s.verifyKeystore = verifyKeystore
	s.cancel = cancel
	s.stopped = make(chan struct{})
	return &s, nil
}

func (s *Service) Stop() {
	s.cancel()
}

func (s *Service) WaitUntilStopped() error {
	<-s.stopped
	return s.stoppedErr
}

func (s *Service) waitUntilStopped() {
	var result error
	err := s.node.ErrGroupWait()
	if err != nil {
		s.log.Error("hybrid exit", zap.Error(err))
		result = multierror.Append(result, err)
	}
	err = s.ipfs.Proccess().Err()
	if err != nil {
		s.log.Error("hybrid exit", zap.Error(err))
		result = multierror.Append(result, err)
	}
	s.log.Sync()
	s.stoppedErr = result
	close(s.stopped)
}
