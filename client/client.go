package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	circuit "github.com/rubyist/circuitbreaker"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"github.com/smallnest/rpcx/util"
)

// ServiceError is an error from server.
type ServiceError string

func (e ServiceError) Error() string {
	return string(e)
}

// DefaultOption is a common option configuration for client.
var DefaultOption = Option{
	Retries:        3,
	RPCPath:        share.DefaultRPCPath,
	ConnectTimeout: 10 * time.Second,
	Breaker:        CircuitBreaker,
	SerializeType:  protocol.MsgPack,
	CompressType:   protocol.None,
}

// Breaker is a CircuitBreaker interface.
type Breaker interface {
	Call(func() error, time.Duration) error
}

// CircuitBreaker is a default circuit breaker (RateBreaker(0.95, 100)).
var CircuitBreaker Breaker = circuit.NewRateBreaker(0.95, 100)

// ErrShutdown connection is closed.
var (
	ErrShutdown         = errors.New("connection is shut down")
	ErrUnsupportedCodec = errors.New("unsupported codec")
)

const (
	// ReaderBuffsize is used for bufio reader.
	ReaderBuffsize = 16 * 1024
	// WriterBuffsize is used for bufio writer.
	WriterBuffsize = 16 * 1024
)

type seqKey struct{}

// RPCClient is interface that defines one client to call one server.
type RPCClient interface {
	Connect(network, address string) error
	Go(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call
	Call(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}) error
	Close() error

	IsClosing() bool
	IsShutdown() bool
}

// Client represents a RPC client.
type Client struct {
	option Option

	Conn net.Conn
	r    *bufio.Reader
	//w    *bufio.Writer

	mutex    sync.Mutex // protects following
	seq      uint64
	pending  map[uint64]*Call
	closing  bool // user has called Close
	shutdown bool // server has told us to stop

	Plugins PluginContainer
}

// Option contains all options for creating clients.
type Option struct {
	// Retries retries to send
	Retries int

	// TLSConfig for tcp and quic
	TLSConfig *tls.Config
	// kcp.BlockCrypt
	Block interface{}
	// RPCPath for http connection
	RPCPath string
	//ConnectTimeout sets timeout for dialing
	ConnectTimeout time.Duration
	// ReadTimeout sets readdeadline for underlying net.Conns
	ReadTimeout time.Duration
	// WriteTimeout sets writedeadline for underlying net.Conns
	WriteTimeout time.Duration

	// Breaker is used to config CircuitBreaker
	Breaker Breaker

	SerializeType protocol.SerializeType
	CompressType  protocol.CompressType

	Heartbeat         bool
	HeartbeatInterval time.Duration
}

// Call represents an active RPC.
type Call struct {
	ServicePath   string            // The name of the service and method to call.
	ServiceMethod string            // The name of the service and method to call.
	Metadata      map[string]string //metadata
	ResMetadata   map[string]string
	Args          interface{} // The argument to the function (*struct).
	Reply         interface{} // The reply from the function (*struct).
	Error         error       // After completion, the error status.
	Done          chan *Call  // Strobes when call is complete.
}

func (call *Call) done() {
	select {
	case call.Done <- call:
		// ok
	default:
		log.Debug("rpc: discarding Call reply due to insufficient Done chan capacity")

	}
}

// IsClosing client is closing or not.
func (client *Client) IsClosing() bool {
	return client.closing
}

// IsShutdown client is shutdown or not.
func (client *Client) IsShutdown() bool {
	return client.shutdown
}

// Go invokes the function asynchronously. It returns the Call structure representing
// the invocation. The done channel will signal when the call is complete by returning
// the same Call object. If done is nil, Go will allocate a new channel.
// If non-nil, done must be buffered or Go will deliberately crash.
func (client *Client) Go(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	call := new(Call)
	call.ServicePath = servicePath
	call.ServiceMethod = serviceMethod
	meta := ctx.Value(share.ReqMetaDataKey)
	if meta != nil { //copy meta in context to meta in requests
		call.Metadata = meta.(map[string]string)
	}
	call.Args = args
	call.Reply = reply
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel. If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("rpc: done channel is unbuffered")
		}
	}
	call.Done = done
	client.send(ctx, call)
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}) error {
	if client.option.Breaker != nil {
		return client.option.Breaker.Call(func() error {
			return client.call(ctx, servicePath, serviceMethod, args, reply)
		}, 0)
	}

	return client.call(ctx, servicePath, serviceMethod, args, reply)
}

func (client *Client) call(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}) error {
	seq := new(uint64)
	ctx = context.WithValue(ctx, seqKey{}, seq)
	Done := client.Go(ctx, servicePath, serviceMethod, args, reply, make(chan *Call, 1)).Done

	var err error
	select {
	case <-ctx.Done(): //cancel by context
		client.mutex.Lock()
		call := client.pending[*seq]
		delete(client.pending, *seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = ctx.Err()
			call.done()
		}

		return ctx.Err()
	case call := <-Done:
		err = call.Error
		meta := ctx.Value(share.ResMetaDataKey)
		if meta != nil && len(call.ResMetadata) > 0 {
			resMeta := meta.(map[string]string)
			for k, v := range call.ResMetadata {
				resMeta[k] = v
			}
		}
	}

	return err
}

func (client *Client) send(ctx context.Context, call *Call) {

	// Register this call.
	client.mutex.Lock()
	if client.shutdown || client.closing {
		call.Error = ErrShutdown
		client.mutex.Unlock()
		call.done()
		return
	}

	codec := share.Codecs[client.option.SerializeType]
	if codec == nil {
		call.Error = ErrUnsupportedCodec
		client.mutex.Unlock()
		call.done()
		return
	}

	if client.pending == nil {
		client.pending = make(map[uint64]*Call)
	}

	seq := client.seq
	client.seq++
	client.pending[seq] = call
	client.mutex.Unlock()

	if cseq, ok := ctx.Value(seqKey{}).(*uint64); ok {
		*cseq = seq
	}

	req := protocol.NewMessage()
	req.SetMessageType(protocol.Request)
	req.SetSeq(seq)
	// heartbeat
	if call.ServicePath == "" && call.ServiceMethod == "" {
		req.SetHeartbeat(true)
	} else {
		req.SetSerializeType(client.option.SerializeType)
		if call.Metadata != nil {
			req.Metadata = call.Metadata
		}

		req.ServicePath = call.ServicePath
		req.ServiceMethod = call.ServiceMethod

		data, err := codec.Encode(call.Args)
		if err != nil {
			call.Error = err
			call.done()
			return
		}

		if len(data) > 1024 && client.option.CompressType == protocol.Gzip {
			data, err = util.Zip(data)
			if err != nil {
				call.Error = err
				call.done()
				return
			}

			req.SetCompressType(client.option.CompressType)
		}

		req.Payload = data
	}

	data := req.Encode()
	_, err := client.Conn.Write(data)
	if err != nil {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.done()
		}
	}

	if req.IsOneway() {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.done()
		}
	}

}

func (client *Client) input() {
	var err error
	var res *protocol.Message
	for err == nil {
		res, err = protocol.Read(client.r)

		if err != nil {
			break
		}
		seq := res.Seq()
		client.mutex.Lock()
		call := client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()

		switch {
		case call == nil:
			// We've got no pending call. That usually means that
			// WriteRequest partially failed, and call was already
			// removed; response is a server telling us about an
			// error reading request body. We should still attempt
			// to read error body, but there's no one to give it to.
		case res.MessageStatusType() == protocol.Error:
			// We've got an error response. Give this to the request;
			call.Error = ServiceError(res.Metadata[protocol.ServiceError])
			call.ResMetadata = res.Metadata
			call.done()
		default:
			data := res.Payload
			if len(data) > 0 {
				if res.CompressType() == protocol.Gzip {
					data, err = util.Unzip(data)
					if err != nil {
						call.Error = ServiceError("unzip payload: " + err.Error())
					}
				}

				codec := share.Codecs[res.SerializeType()]
				if codec == nil {
					call.Error = ServiceError(ErrUnsupportedCodec.Error())
				} else {
					err = codec.Decode(data, call.Reply)
					if err != nil {
						call.Error = ServiceError(err.Error())
					}
				}
			}
			call.ResMetadata = res.Metadata
			call.done()
		}
	}
	// Terminate pending calls.
	client.mutex.Lock()
	client.shutdown = true
	closing := client.closing
	if err == io.EOF {
		if closing {
			err = ErrShutdown
		} else {
			err = io.ErrUnexpectedEOF
		}
	}
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
	client.mutex.Unlock()
	if err != io.EOF && !closing {
		log.Error("rpcx: client protocol error:", err)
	}
}

func (client *Client) heartbeat() {
	t := time.NewTicker(client.option.HeartbeatInterval)

	for range t.C {
		if client.shutdown || client.closing {
			return
		}

		err := client.Call(context.Background(), "", "", nil, nil)
		if err != nil {
			log.Warnf("failed to heartbeat to %s", client.Conn.RemoteAddr().String())
		}
	}
}

// Close calls the underlying codec's Close method. If the connection is already
// shutting down, ErrShutdown is returned.
func (client *Client) Close() error {
	client.mutex.Lock()

	for seq, call := range client.pending {
		delete(client.pending, seq)
		if call != nil {
			call.Error = ErrShutdown
			call.done()
		}
	}

	if client.closing || client.shutdown {
		client.mutex.Unlock()
		return ErrShutdown
	}

	client.closing = true
	client.mutex.Unlock()
	return client.Conn.Close()
}
