package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	circuit "github.com/rubyist/circuitbreaker"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"github.com/smallnest/rpcx/util"
)

const (
	XVersion           = "X-RPCX-Version"
	XMessageType       = "X-RPCX-MesssageType"
	XHeartbeat         = "X-RPCX-Heartbeat"
	XOneway            = "X-RPCX-Oneway"
	XMessageStatusType = "X-RPCX-MessageStatusType"
	XSerializeType     = "X-RPCX-SerializeType"
	XMessageID         = "X-RPCX-MessageID"
	XServicePath       = "X-RPCX-ServicePath"
	XServiceMethod     = "X-RPCX-ServiceMethod"
	XMeta              = "X-RPCX-Meta"
	XErrorMessage      = "X-RPCX-ErrorMessage"
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
	SerializeType:  protocol.MsgPack,
	CompressType:   protocol.None,
	BackupLatency:  10 * time.Millisecond,
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
	SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error)
	Close() error

	RegisterServerMessageChan(ch chan<- *protocol.Message)
	UnregisterServerMessageChan()

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

	ServerMessageChan chan<- *protocol.Message
}

// NewClient returns a new Client with the option.
func NewClient(option Option) *Client {
	return &Client{
		option: option,
	}
}

// Option contains all options for creating clients.
type Option struct {
	// Group is used to select the services in the same group. Services set group info in their meta.
	// If it is empty, clients will ignore group.
	Group string

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

	// BackupLatency is used for Failbackup mode. rpcx will sends another request if the first response doesn't return in BackupLatency time.
	BackupLatency time.Duration

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
	Raw           bool        // raw message or not
}

func (call *Call) done() {
	select {
	case call.Done <- call:
		// ok
	default:
		log.Debug("rpc: discarding Call reply due to insufficient Done chan capacity")

	}
}

// RegisterServerMessageChan registers the channel that receives server requests.
func (client *Client) RegisterServerMessageChan(ch chan<- *protocol.Message) {
	client.ServerMessageChan = ch
}

// UnregisterServerMessageChan removes ServerMessageChan.
func (client *Client) UnregisterServerMessageChan() {
	client.ServerMessageChan = nil
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

// SendRaw sends raw messages. You don't care args and replys.
func (client *Client) SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error) {
	ctx = context.WithValue(ctx, seqKey{}, r.Seq())

	call := new(Call)
	call.Raw = true
	call.ServicePath = r.ServicePath
	call.ServiceMethod = r.ServiceMethod
	meta := ctx.Value(share.ReqMetaDataKey)
	if meta != nil { //copy meta in context to meta in requests
		call.Metadata = meta.(map[string]string)
	}
	done := make(chan *Call, 10)
	call.Done = done

	seq := r.Seq()
	client.mutex.Lock()
	if client.pending == nil {
		client.pending = make(map[uint64]*Call)
	}
	client.pending[seq] = call
	client.mutex.Unlock()

	data := r.Encode()
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
	if r.IsOneway() {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.done()
		}
	}

	var m map[string]string
	var payload []byte

	select {
	case <-ctx.Done(): //cancel by context
		client.mutex.Lock()
		call := client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = ctx.Err()
			call.done()
		}

		return nil, nil, ctx.Err()
	case call := <-done:
		err = call.Error
		m = call.Metadata
		if call.Reply != nil {
			payload = call.Reply.([]byte)
		}
	}

	return m, payload, err
}

func convertRes2Raw(res *protocol.Message) (map[string]string, []byte, error) {
	m := make(map[string]string)
	m[XVersion] = strconv.Itoa(int(res.Version()))
	if res.IsHeartbeat() {
		m[XHeartbeat] = "true"
	}
	if res.IsOneway() {
		m[XOneway] = "true"
	}
	if res.MessageStatusType() == protocol.Error {
		m[XMessageStatusType] = "Error"
	} else {
		m[XMessageStatusType] = "Normal"
	}

	if res.CompressType() == protocol.Gzip {
		m["Content-Encoding"] = "gzip"
	}

	m[XMeta] = urlencode(res.Metadata)
	m[XSerializeType] = strconv.Itoa(int(res.SerializeType()))
	m[XMessageID] = strconv.FormatUint(res.Seq(), 10)
	m[XServicePath] = res.ServicePath
	m[XServiceMethod] = res.ServiceMethod

	return m, res.Payload, nil
}

func urlencode(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	var buf bytes.Buffer
	for k, v := range data {
		buf.WriteString(url.QueryEscape(k))
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(v))
		buf.WriteByte('&')
	}
	s := buf.String()
	return s[0 : len(s)-1]
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

	//req := protocol.NewMessage()
	req := protocol.GetPooledMsg()
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

	isOneway := req.IsOneway()
	protocol.FreeMsg(req)

	if isOneway {
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
	var res = protocol.NewMessage()

	for err == nil {
		err = res.Decode(client.r)
		//res, err = protocol.Read(client.r)

		if err != nil {
			break
		}
		seq := res.Seq()
		var call *Call
		isServerMessage := (res.MessageType() == protocol.Request && !res.IsHeartbeat() && res.IsOneway())
		if !isServerMessage {
			client.mutex.Lock()
			call = client.pending[seq]
			delete(client.pending, seq)
			client.mutex.Unlock()
		}

		switch {
		case call == nil:
			if isServerMessage {
				if client.ServerMessageChan != nil {
					go client.handleServerRequest(res)
				}
				continue
			}
		case res.MessageStatusType() == protocol.Error:
			// We've got an error response. Give this to the request;
			call.Error = ServiceError(res.Metadata[protocol.ServiceError])
			call.ResMetadata = res.Metadata

			if call.Raw {
				call.Metadata, call.Reply, _ = convertRes2Raw(res)
				call.Metadata[XErrorMessage] = call.Error.Error()
			}
			call.done()
		default:
			if call.Raw {
				call.Metadata, call.Reply, _ = convertRes2Raw(res)
			} else {
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
			}

			call.done()
		}

		res.Reset()
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
	if err != nil && err != io.EOF && !closing {
		log.Error("rpcx: client protocol error:", err)
	}
}

func (client *Client) handleServerRequest(msg *protocol.Message) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("ServerMessageChan may be closed so client remove it. Please add it again if you want to handle server requests. error is %v", r)
			client.ServerMessageChan = nil
		}
	}()

	t := time.NewTimer(5 * time.Second)
	select {
	case client.ServerMessageChan <- msg:
	case <-t.C:
		log.Warnf("ServerMessageChan may be full so the server request %d has been dropped", msg.Seq())
	}
	t.Stop()
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
