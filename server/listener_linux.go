//go:build linux
// +build linux

package server

import (
	"net"
	"runtime"
	"time"

	uringnet "github.com/godzie44/go-uring/net"
	"github.com/godzie44/go-uring/reactor"
	"github.com/godzie44/go-uring/uring"
	"github.com/smallnest/rpcx/log"
)

func init() {
	makeListeners["iouring"] = iouringMakeListener
}

// iouringMakeListener creates a new listener using io_uring.
// You can use RegisterMakeListener to register a customized iouring Listener creator.
// experimental
func iouringMakeListener(s *Server, address string) (ln net.Listener, err error) {
	n := runtime.GOMAXPROCS(-1)

	var opts []uring.SetupOption
	opts = append(opts, uring.WithSQPoll(time.Millisecond*100))

	rings, closeRings, err := uring.CreateMany(n, uring.MaxEntries>>3, n, opts...)
	if err != nil {
		return nil, err
	}

	netReactor, err := reactor.NewNet(rings, reactor.WithLogger(&uringLogger{log.GetLogger()}))
	if err != nil {
		return nil, err
	}

	ln, err = uringnet.NewListener(net.ListenConfig{}, address, netReactor)
	if err != nil {
		return nil, err
	}

	return &uringnetListener{Listener: ln, closeRings: closeRings}, nil
}

type uringnetListener struct {
	net.Listener
	closeRings uring.Defer
}

func (cl *uringnetListener) Close() error {
	cl.closeRings()
	cl.Listener.Close()

	return nil
}

type uringLogger struct {
	log.Logger
}

func (l *uringLogger) Log(keyvals ...interface{}) {
	l.Logger.Info(keyvals...)
}
