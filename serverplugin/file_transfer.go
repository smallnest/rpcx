package serverplugin

import (
	"context"
	"crypto/rand"
	"io"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/server"
)

var (
	SendFileServiceName = "_filetransfer"
)

// FileTransferHandler handles uploading file. Must close the connection after it finished.
type FileTransferHandler func(conn net.Conn, args *FileTransferArgs)

// DownloadFileHandler handles downloading file. Must close the connection after it finished.
type DownloadFileHandler func(conn net.Conn, args *DownloadFileArgs)

// FileTransferArgs args from clients.
type FileTransferArgs struct {
	FileName string            `json:"file_name,omitempty"`
	FileSize int64             `json:"file_size,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

// FileTransferReply response to token and addr to clients.
type FileTransferReply struct {
	Token []byte `json:"token,omitempty"`
	Addr  string `json:"addr,omitempty"`
}

// DownloadFileArgs args from clients.
type DownloadFileArgs struct {
	FileName string `json:"file_name,omitempty"`
}

type tokenInfo struct {
	token []byte
	args  *FileTransferArgs
}

type downloadTokenInfo struct {
	token []byte
	args  *DownloadFileArgs
}

// FileTransfer support transfer files from clients.
// It registers a file transfer service and listens a on the given port.
// Clients will invokes this service to get the token and send the token and the file to this port.
type FileTransfer struct {
	Addr                string
	handler             FileTransferHandler
	downloadFileHandler DownloadFileHandler
	cachedTokens        *lru.Cache
	service             *FileTransferService

	startOnce sync.Once

	done chan struct{}
}

type FileTransferService struct {
	FileTransfer *FileTransfer
}

// NewFileTransfer creates a FileTransfer with given parameters.
func NewFileTransfer(addr string, handler FileTransferHandler, downloadFileHandler DownloadFileHandler, waitNum int) *FileTransfer {

	cachedTokens, _ := lru.New(waitNum)

	fi := &FileTransfer{
		Addr:                addr,
		handler:             handler,
		downloadFileHandler: downloadFileHandler,
		cachedTokens:        cachedTokens,
	}

	fi.service = &FileTransferService{
		FileTransfer: fi,
	}

	return fi
}

// RegisterFileTransfer register filetransfer service into the server.
func RegisterFileTransfer(s *server.Server, fileTransfer *FileTransfer) {
	fileTransfer.Start()
	s.RegisterName(SendFileServiceName, fileTransfer.service, "")
}

func (s *FileTransferService) TransferFile(ctx context.Context, args *FileTransferArgs, reply *FileTransferReply) error {
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return err
	}

	*reply = FileTransferReply{
		Token: token,
		Addr:  s.FileTransfer.Addr,
	}

	s.FileTransfer.cachedTokens.Add(string(token), &tokenInfo{token, args})
	return nil
}

func (s *FileTransferService) DownloadFile(ctx context.Context, args *DownloadFileArgs, reply *FileTransferReply) error {
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return err
	}

	*reply = FileTransferReply{
		Token: token,
		Addr:  s.FileTransfer.Addr,
	}

	s.FileTransfer.cachedTokens.Add(string(token), &downloadTokenInfo{token, args})
	return nil
}

func (s *FileTransfer) Start() error {
	s.startOnce.Do(func() {
		go s.start()
	})

	return nil
}

func (s *FileTransfer) start() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}

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
			case *tokenInfo:
				if s.handler == nil {
					conn.Close()
					continue
				}
				go s.handler(conn, ti.args)
			case *downloadTokenInfo:
				if s.downloadFileHandler == nil {
					conn.Close()
					continue
				}
				go s.downloadFileHandler(conn, ti.args)
			default:
				conn.Close()
			}

		}

	}
}

func (s *FileTransfer) Stop() error {
	close(s.done)

	return nil
}
