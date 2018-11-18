// gomobile must be created by developer. The package name must be gomobile.
// These apis must/only be here: ConcurrentRunner, FromGo, NewFromGo.
// For ios, do not forget add "${PODS_ROOT}/../Frameworks" to Framework search
// path. Use `make ios` and  `make android`.
package gomobile

import (
	"context"
	"errors"
	"log"
	"math"
	"net"
	"strconv"

	"github.com/empirefox/flutter_dial_go/go/forgo"
	"github.com/empirefox/hybrid/grpc"
	"github.com/empirefox/hybrid/pkg/zapsuit"
)

type ConcurrentRunner interface {
	// Done will wait the runner to end.
	// Safe to be called multi times with multi threads.
	Done() error
}

type FromGo interface {
	// DoInitOnce do init things like listening. Do clean internally if err.
	// Safe to be called multi times with multi threads.
	DoInitOnce() ConcurrentRunner

	// DoDestroyOnce do clean things like freeing resources, closing listeners.
	// Safe to be called multi times with multi threads.
	DoDestroyOnce() ConcurrentRunner
}

func NewFromGo() FromGo {
	return &fromGo{
		init:    forgo.NewConcurrentRunner(),
		destroy: forgo.NewConcurrentRunner(),
	}
}

var (
	errInvalidPort = errors.New("invalid port")
)

type fromGo struct {
	init    *forgo.ConcurrentRunner
	destroy *forgo.ConcurrentRunner

	server *grpc.Server
	cancel context.CancelFunc
}

func (m *fromGo) DoInitOnce() ConcurrentRunner {
	m.init.Once(func() error {
		err := zapsuit.RegisterTCPSink()
		if err != nil {
			log.Printf("register zap TCP sink: %v", err)
			return err
		}

		const port uint16 = 1212
		ln, err := forgo.Listen(port)
		if err != nil {
			log.Printf("listener on %d err: %v", port, err)
			return err
		}
		defer func() {
			if m.server == nil {
				ln.Close()
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			if m.server == nil {
				cancel()
			}
		}()

		server, err := grpc.NewServer(grpc.Config{
			Context: ctx,
			Listen: func(network, address string) (net.Listener, error) {
				switch network {
				case "fdg":
					port, err := strconv.ParseUint(address, 10, 32)
					if err != nil {
						return nil, err
					}
					if port == 0 || port > math.MaxUint16 {
						return nil, errInvalidPort
					}
					return forgo.Listen(uint16(port))
				default:
					return net.Listen(network, address)
				}
			},
			MinVerifyKeyLife: 0, // use default
			MaxVerifyKeyLife: 0, // use default
			BindSeqStart:     0,
			ConfigBindId:     0,
		})
		if err != nil {
			log.Printf("grpc.NewServer: %v", err)
			return err
		}
		defer func() {
			if m.server == nil {
				server.Stop(context.Background(), nil)
			}
		}()

		go func() {
			err := server.ServeGrpc(ln)
			if err != nil {
				log.Printf("ServeGrpc err: %v", err)
			}
			cancel()
		}()

		m.server = server
		m.cancel = cancel
		return nil
	})
	return m.init
}

func (m *fromGo) DoDestroyOnce() ConcurrentRunner {
	m.destroy.Once(func() error {
		m.cancel()
		_, err := m.server.WaitUntilStopped(context.Background(), nil)
		return err
	})
	return m.destroy
}
