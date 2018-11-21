package grpc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/authstore"
	"github.com/empirefox/hybrid/pkg/ipfs"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	// DefaultMinVerifyKeyLife = 1hour
	DefaultMinVerifyKeyLife = 3600

	// DefaultMaxVerifyKeyLife = 365days
	DefaultMaxVerifyKeyLife = 31536000
)

var (
	ErrKeyLife               = errors.New("invalid key life")
	ErrConfigBindId          = errors.New("invalid ConfigBindId")
	ErrNoService             = errors.New("no service")
	ErrServiceAlreadyStarted = errors.New("service already started")
	ErrIpfsRepoLocked        = errors.New("ipfs repo locked")
	ErrNotImplememted        = errors.New("grpc api not implemented")
)

type ListenFunc func(network, address string) (net.Listener, error)

type Config struct {
	// Context is required.
	Context context.Context

	// Listen is required.
	Listen ListenFunc

	// MinVerifyKeyLife is second
	MinVerifyKeyLife uint32

	// MaxVerifyKeyLife is second
	MaxVerifyKeyLife uint32

	BindSeqStart uint32

	// ConfigBindId must not greater than BindSeqStart
	ConfigBindId uint32
}

type Server struct {
	config  Config
	service *Service
	bindSeq uint32
	mu      sync.Mutex
}

func NewServer(c Config) (*Server, error) {
	if c.MaxVerifyKeyLife < c.MinVerifyKeyLife {
		return nil, ErrKeyLife
	}
	if c.ConfigBindId > c.BindSeqStart {
		return nil, ErrConfigBindId
	}
	return &Server{
		config:  c,
		bindSeq: c.BindSeqStart,
	}, nil
}

func (s *Server) ServeGrpc(ln net.Listener) error {
	gs := grpc.NewServer()
	RegisterHybridServer(gs, s)
	reflection.Register(gs)
	return gs.Serve(ln)
}

func (s *Server) GetVersion(_ context.Context, _ *empty.Empty) (*Version, error) {
	return GetVersion(), nil
}
func (s *Server) GetConfigTree(_ context.Context, req *StartRequest) (*config.ConfigTree, error) {
	return config.NewTree(req.Root)
}

func (s *Server) Start(ctx context.Context, req *StartRequest) (_ *empty.Empty, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service != nil {
		return nil, ErrServiceAlreadyStarted
	}
	// not ctx from argument
	s.service, err = Start(s.config.Context, req.Root, s.config.ConfigBindId)
	if err == nil && s.service.config.Bind != "" {
		err = s.service.node.StartConfigProxy()
	}
	return
}
func (s *Server) Stop(_ context.Context, _ *empty.Empty) (*empty.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service != nil {
		s.service.Stop()
	}
	return nil, nil
}
func (s *Server) WaitUntilStopped(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	s.mu.Lock()
	if s.service == nil {
		s.mu.Unlock()
		return nil, nil
	}
	service := s.service
	s.mu.Unlock()

	errc := make(chan error)
	go func() {
		err := service.WaitUntilStopped()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.service == service {
			s.service = nil
		}
		errc <- err
	}()

	select {
	case err := <-errc:
		return nil, err
	case <-ctx.Done():
		return nil, nil
	}
}

func (s *Server) BindConfigProxy(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, ErrNoService
	}

	return nil, s.service.node.StartConfigProxy()
}
func (s *Server) UnbindConfigProxy(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, nil
	}

	return nil, s.service.node.StopConfigProxy()
}

func (s *Server) BindProxy(_ context.Context, req *BindRequest) (*BindData, error) {
	return s.doBind(req, s.service.node.StartProxy)
}
func (s *Server) BindIpfsApi(_ context.Context, req *BindRequest) (*BindData, error) {
	return s.doBind(req, s.service.node.StartIpfsApi)
}
func (s *Server) BindIpfsGateway(_ context.Context, req *BindRequest) (*BindData, error) {
	return s.doBind(req, s.service.node.StartIpfsGateway)
}

func (s *Server) doBind(req *BindRequest, startServe func(uint32, net.Listener)) (*BindData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, ErrNoService
	}

	ln, err := s.config.Listen(req.Network, req.Address)
	if err != nil {
		return nil, err
	}

	s.bindSeq++
	startServe(s.bindSeq, ln)
	return &BindData{Bind: s.bindSeq}, nil
}

func (s *Server) Unbind(_ context.Context, req *BindData) (*empty.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, ErrNoService
	}
	return nil, s.service.node.StopListener(req.Bind)
}

func (s *Server) IpfsRepoFsck(_ context.Context, req *StartRequest) (*empty.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rootPath, err := filepath.Abs(os.ExpandEnv(req.Root))
	if err != nil {
		return nil, err
	}

	if s.service != nil && rootPath == s.service.config.Tree().ConfigPath {
		return nil, ErrIpfsRepoLocked
	}
	return nil, ipfs.Fsck(rootPath)
}

func (s *Server) Backup(ctx context.Context, req *BackupRequest) (*empty.Empty, error) {
	return nil, ErrNotImplememted
}
func (s *Server) Restore(ctx context.Context, req *BackupRequest) (*empty.Empty, error) {
	return nil, ErrNotImplememted
}

func (s *Server) AddVerifyKey(_ context.Context, req *AddVerifyKeyRequest) (*AddVerifyKeyReply, error) {
	life := req.LifeSeconds
	if life > s.config.MaxVerifyKeyLife {
		life = s.config.MaxVerifyKeyLife
	}
	if life < s.config.MinVerifyKeyLife {
		life = s.config.MinVerifyKeyLife
	}
	now := time.Now().Unix()
	ak := authstore.AuthKey{
		Key:       req.Key,
		Tags:      req.Tags,
		Desc:      req.Desc,
		CreatedAt: now,
		ExpiresAt: now + int64(life),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, ErrNoService
	}

	err := s.service.verifyKeystore.Save(&ak)
	if err != nil {
		return nil, err
	}

	return &AddVerifyKeyReply{
		Id:        ak.Id,
		CreatedAt: ak.CreatedAt,
		ExpiresAt: ak.ExpiresAt,
	}, nil
}
func (s *Server) GetVerifyKeys(_ context.Context, req *VerifyKeySliceRequest) (*AuthKeySliceReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, ErrNoService
	}

	aks, err := s.service.verifyKeystore.Slice(req.Start, int(req.Size), req.Reverse)
	var errmsg string
	if err != nil {
		errmsg = err.Error()
	}
	return &AuthKeySliceReply{
		Keys: aks,
		Err:  errmsg,
	}, nil
}
func (s *Server) FindVerifyKey(_ context.Context, req *VerifyKeyIdRequest) (*authstore.AuthKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, ErrNoService
	}
	return s.service.verifyKeystore.Find(req.Id)
}
func (s *Server) DeleteVerifyKey(_ context.Context, req *VerifyKeyIdRequest) (*empty.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil, ErrNoService
	}
	return nil, s.service.verifyKeystore.Delete(req.Id)
}
