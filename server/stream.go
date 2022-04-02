package server

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/share"
)

var ErrNotAccept = errors.New("server refused the connection")

// StreamHandler handles a streaming connection with client.
type StreamHandler func(conn net.Conn, args *share.StreamServiceArgs)

// StreamAcceptor accepts connection from clients or not.
// You can use it to validate clients and determine if accept or drop the connection.
type StreamAcceptor func(ctx context.Context, args *share.StreamServiceArgs) bool

type streamTokenInfo struct {
	token []byte
	args  *share.StreamServiceArgs
}

// StreamService support streaming between clients and server.
// It registers a streaming service and listens on the given port.
// Clients will invokes this service to get the token and send the token and begin to stream.
type StreamService struct {
	Addr          string
	AdvertiseAddr string
	handler       StreamHandler
	acceptor      StreamAcceptor
	cachedTokens  *lru.Cache

	startOnce sync.Once

	ln   net.Listener
	done chan struct{}
}

// NewStreamService creates a stream service.
func NewStreamService(addr string, streamHandler StreamHandler, acceptor StreamAcceptor, waitNum int) *StreamService {
	cachedTokens, _ := lru.New(waitNum)

	fi := &StreamService{
		Addr:         addr,
		handler:      streamHandler,
		cachedTokens: cachedTokens,
	}

	return fi
}

// EnableFileTransfer supports filetransfer service in this server.
func (s *Server) EnableStreamService(serviceName string, streamService *StreamService) {
	if serviceName == "" {
		serviceName = share.StreamServiceName
	}
	_ = streamService.Start()
	_ = s.RegisterName(serviceName, streamService, "")
}

func (s *StreamService) Stream(ctx context.Context, args *share.StreamServiceArgs, reply *share.StreamServiceReply) error {
	// clientConn := ctx.Value(server.RemoteConnContextKey).(net.Conn)

	if s.acceptor != nil && !s.acceptor(ctx, args) {
		return ErrNotAccept
	}

	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return err
	}

	*reply = share.StreamServiceReply{
		Token: token,
		Addr:  s.Addr,
	}
	if s.AdvertiseAddr != "" {
		reply.Addr = s.AdvertiseAddr
	}

	s.cachedTokens.Add(string(token), &streamTokenInfo{token, args})
	return nil
}

func (s *StreamService) Start() error {
	s.startOnce.Do(func() {
		go s.start()
	})

	return nil
}

func (s *StreamService) start() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.ln = ln

	var tempDelay time.Duration

	for {
		select {
		case <-s.done:
			return nil
		default:
			conn, e := ln.Accept()
			if e != nil {
				if ne, ok := e.(net.Error); ok && ne.Temporary() {
					if tempDelay == 0 {
						tempDelay = 5 * time.Millisecond
					} else {
						tempDelay *= 2
					}

					if max := 1 * time.Second; tempDelay > max {
						tempDelay = max
					}

					log.Errorf("filetransfer: accept error: %v; retrying in %v", e, tempDelay)
					time.Sleep(tempDelay)
					continue
				}
				return e
			}
			tempDelay = 0

			if tc, ok := conn.(*net.TCPConn); ok {
				tc.SetKeepAlive(true)
				tc.SetKeepAlivePeriod(3 * time.Minute)
				tc.SetLinger(10)
			}

			token := make([]byte, 32)
			_, err := io.ReadFull(conn, token)
			if err != nil {
				conn.Close()
				log.Errorf("failed to read token from %s", conn.RemoteAddr().String())
				continue
			}

			tokenStr := string(token)
			info, ok := s.cachedTokens.Get(tokenStr)
			if !ok {
				conn.Close()
				log.Errorf("failed to read token from %s", conn.RemoteAddr().String())
				continue
			}
			s.cachedTokens.Remove(tokenStr)

			switch ti := info.(type) {
			case *streamTokenInfo:
				if s.handler == nil {
					conn.Close()
					continue
				}
				go s.handler(conn, ti.args)
			default:
				conn.Close()
			}

		}
	}
}

func (s *StreamService) Stop() error {
	close(s.done)
	if s.ln != nil {
		s.ln.Close()
	}
	return nil
}
