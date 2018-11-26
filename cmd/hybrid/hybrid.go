// visit client:
// http://with.0.hybrid/index.html
//
// visit server:
// http://over.server_name_in_hybrid_json.hybrid/index.html
//
// visit host from special server:
// http://192.168.2.6.over.server_name_in_hybrid_json.hybrid/index.html
//
// ssh over hybrid example:
// ssh root@192.168.1.1 -o "ProxyCommand=nc -n -Xconnect -x127.0.0.1:7777 %h.over.server_name_in_hybrid_json.hybrid %p"
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/empirefox/hybrid/grpc"
	"github.com/empirefox/hybrid/pkg/zapsuit"
)

func main() {
	err := zapsuit.RegisterTCPSink()
	if err != nil {
		log.Fatalf("register zap TCP sink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	intrh, ctx := setupInterruptHandler(ctx)
	defer intrh.Close()

	s, err := grpc.NewServer(grpc.Config{
		Context:          ctx,
		Listen:           net.Listen,
		MinVerifyKeyLife: 0, // use default
		MaxVerifyKeyLife: 0, // use default
		BindSeqStart:     0,
		ConfigBindId:     0,
	})
	if err != nil {
		log.Fatalf("grpc.NewServer: %v", err)
	}

	// use https://github.com/ktr0731/evans to add verify keys
	grpcBind := os.Getenv("HYBRID_GRPC_BIND")
	if grpcBind != "" {
		ln, err := net.Listen("tcp", grpcBind)
		if err != nil {
			log.Fatalf("listener on %s err: %v", grpcBind, err)
		}
		go func() {
			err := s.ServeGrpc(ln)
			if err != nil {
				log.Printf("ServeGrpc err: %v", err)
			}
			cancel()
		}()
	}

	// $HYBRID_ROOT_PATH or $HOME/.hybrid
	_, err = s.Start(context.Background(), &grpc.StartRequest{Root: os.Getenv("HYBRID_ROOT_PATH")})
	if err != nil {
		if grpcBind != "" {
			log.Printf("check the config, then start from grpc client.\n grpc.Server.Start: %v\n", err)
		} else {
			log.Fatalf("grpc.Server.Start: %v", err)
		}
	}

	log.Printf("Hybrid started!")
	<-ctx.Done()
	s.WaitUntilStopped(context.Background(), nil)
}

// IntrHandler helps set up an interrupt handler that can
// be cleanly shut down through the io.Closer interface.
type IntrHandler struct {
	sig chan os.Signal
	wg  sync.WaitGroup
}

func NewIntrHandler() *IntrHandler {
	ih := &IntrHandler{}
	ih.sig = make(chan os.Signal, 1)
	return ih
}

func (ih *IntrHandler) Close() error {
	close(ih.sig)
	ih.wg.Wait()
	return nil
}

// Handle starts handling the given signals, and will call the handler
// callback function each time a signal is catched. The function is passed
// the number of times the handler has been triggered in total, as
// well as the handler itself, so that the handling logic can use the
// handler's wait group to ensure clean shutdown when Close() is called.
func (ih *IntrHandler) Handle(handler func(count int, ih *IntrHandler), sigs ...os.Signal) {
	signal.Notify(ih.sig, sigs...)
	ih.wg.Add(1)
	go func() {
		defer ih.wg.Done()
		count := 0
		for range ih.sig {
			count++
			handler(count, ih)
		}
		signal.Stop(ih.sig)
	}()
}

func setupInterruptHandler(ctx context.Context) (io.Closer, context.Context) {
	intrh := NewIntrHandler()
	ctx, cancelFunc := context.WithCancel(ctx)

	handlerFunc := func(count int, ih *IntrHandler) {
		switch count {
		case 1:
			fmt.Println() // Prevent un-terminated ^C character in terminal

			ih.wg.Add(1)
			go func() {
				defer ih.wg.Done()
				cancelFunc()
			}()

		default:
			fmt.Println("Received another interrupt before graceful shutdown, terminating...")
			os.Exit(-1)
		}
	}

	intrh.Handle(handlerFunc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	return intrh, ctx
}
