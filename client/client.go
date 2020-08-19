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

	opentracing "github.com/opentracing/opentracing-go"
	circuit "github.com/rubyist/circuitbreaker"
	"github.com/smallnest/rpcx/v5/log"
	"github.com/smallnest/rpcx/v5/protocol"
	"github.com/smallnest/rpcx/v5/share"
	"go.opencensus.io/trace"
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
	Fail()
	Success()
	Ready() bool
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

	mutex        sync.Mutex // protects following
	seq          uint64
	pending      map[uint64]*Call
	closing      bool // user has called Close
	shutdown     bool // server has told us to stop
	pluginClosed bool // the plugin has been called

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
	GenBreaker func() Breaker

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
	client.mutex.Lock()
	defer client.mutex.Unlock()
	return client.closing
}

// IsShutdown client is shutdown or not.
func (client *Client) IsShutdown() bool {
	client.mutex.Lock()
	defer client.mutex.Unlock()
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

	if _, ok := ctx.(*share.Context); !ok {
		ctx = share.NewContext(ctx)
	}

	// TODO: should implement as plugin
	client.injectOpenTracingSpan(ctx, call)
	client.injectOpenCensusSpan(ctx, call)

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

func (client *Client) injectOpenTracingSpan(ctx context.Context, call *Call) {
	var rpcxContext *share.Context
	var ok bool
	if rpcxContext, ok = ctx.(*share.Context); !ok {
		return
	}
	sp := rpcxContext.Value(share.OpentracingSpanClientKey)
	if sp == nil { // have not config opentracing plugin
		return
	}

	span := sp.(opentracing.Span)
	if call.Metadata == nil {
		call.Metadata = make(map[string]string)
	}
	meta := call.Metadata

	err := opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.TextMap,
		opentracing.TextMapCarrier(meta))
	if err != nil {
		log.Errorf("failed to inject span: %v", err)
	}
}

func (client *Client) injectOpenCensusSpan(ctx context.Context, call *Call) {
	var rpcxContext *share.Context
	var ok bool
	if rpcxContext, ok = ctx.(*share.Context); !ok {
		return
	}
	sp := rpcxContext.Value(share.OpencensusSpanClientKey)
	if sp == nil { // have not config opencensus plugin
		return
	}

	span := sp.(*trace.Span)
	if span == nil {
		return
	}
	if call.Metadata == nil {
		call.Metadata = make(map[string]string)
	}
	meta := call.Metadata

	spanContext := span.SpanContext()
	scData := make([]byte, 24)
	copy(scData[:16], spanContext.TraceID[:])
	copy(scData[16:24], spanContext.SpanID[:])
	meta[share.OpencensusSpanRequestKey] = string(scData)
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}) error {
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

	rmeta := make(map[string]string)

	// copy meta to rmeta
	if meta != nil {
		for k, v := range meta.(map[string]string) {
			rmeta[k] = v
		}
	}
	// copy r.Metadata to rmeta
	if r.Metadata != nil {
		for k, v := range r.Metadata {
			rmeta[k] = v
		}
	}

	if meta != nil { //copy meta in context to meta in requests
		call.Metadata = rmeta
	}
	r.Metadata = rmeta

	done := make(chan *Call, 10)
	call.Done = done

	seq := r.Seq()
	client.mutex.Lock()
	if client.pending == nil {
		client.pending = make(map[uint64]*Call)
	}
	client.pending[seq] = call
	client.mutex.Unlock()

	data := r.EncodeSlicePointer()
	_, err := client.Conn.Write(*data)
	protocol.PutData(data)

	if err != nil {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.done()
		}
		return nil, nil, err
	}
	if r.IsOneway() {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.done()
		}
		return nil, nil, nil
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
	if call.Reply == nil {
		req.SetOneway(true)
	}

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
			delete(client.pending, seq)
			call.Error = err
			call.done()
			return
		}
		if len(data) > 1024 && client.option.CompressType != protocol.None {
			req.SetCompressType(client.option.CompressType)
		}

		req.Payload = data
	}

	if client.Plugins != nil {
		client.Plugins.DoClientBeforeEncode(req)
	}

	data := req.EncodeSlicePointer()
	_, err := client.Conn.Write(*data)
	protocol.PutData(data)

	if err != nil {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.done()
		}
		protocol.FreeMsg(req)
		return
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

	if client.option.WriteTimeout != 0 {
		client.Conn.SetWriteDeadline(time.Now().Add(client.option.WriteTimeout))
	}

}

func (client *Client) input() {
	var err error

	for err == nil {
		var res = protocol.NewMessage()
		if client.option.ReadTimeout != 0 {
			client.Conn.SetReadDeadline(time.Now().Add(client.option.ReadTimeout))
		}

		err = res.Decode(client.r)
		if err != nil {
			break
		}
		if client.Plugins != nil {
			client.Plugins.DoClientAfterDecode(res)
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
			// We've got an error response. Give this to the request
			if len(res.Metadata) > 0 {
				call.ResMetadata = res.Metadata
				call.Error = ServiceError(res.Metadata[protocol.ServiceError])
			}

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
				if len(res.Metadata) > 0 {
					call.ResMetadata = res.Metadata
				}

			}

			call.done()
		}
	}
	// Terminate pending calls.

	if client.ServerMessageChan != nil {
		req := protocol.NewMessage()
		req.SetMessageType(protocol.Request)
		req.SetMessageStatusType(protocol.Error)
		if req.Metadata == nil {
			req.Metadata = make(map[string]string)
			if err != nil {
				req.Metadata[protocol.ServiceError] = err.Error()
			}
		}
		req.Metadata["server"] = client.Conn.RemoteAddr().String()
		go client.handleServerRequest(req)
	}

	client.mutex.Lock()
	if !client.pluginClosed {
		if client.Plugins != nil {
			client.Plugins.DoClientConnectionClose(client.Conn)
		}
		client.pluginClosed = true
	}
	client.Conn.Close()
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
		if client.IsShutdown() || client.IsClosing() {
			t.Stop()
			return
		}

		err := client.Call(context.Background(), "", "", nil, nil)
		if err != nil {
			log.Warnf("failed to heartbeat to %s", client.Conn.RemoteAddr().String())
		}
	}
}

// Close calls the underlying connection's Close method. If the connection is already
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

	var err error
	if !client.pluginClosed {
		if client.Plugins != nil {
			client.Plugins.DoClientConnectionClose(client.Conn)
		}

		client.pluginClosed = true
		err = client.Conn.Close()
	}

	if client.closing || client.shutdown {
		client.mutex.Unlock()
		return ErrShutdown
	}

	client.closing = true
	client.mutex.Unlock()
	return err
}
