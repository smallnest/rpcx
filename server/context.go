package server

import (
	"fmt"
	"net"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
)

// Context represents a rpcx FastCall context.
type Context struct {
	conn net.Conn
	req  *protocol.Message
	ctx  *share.Context

	writeCh chan *[]byte
}

// NewContext creates a server.Context for Handler.
func NewContext(ctx *share.Context, conn net.Conn, req *protocol.Message, writeCh chan *[]byte) *Context {
	return &Context{conn: conn, req: req, ctx: ctx, writeCh: writeCh}
}

// Get returns value for key.
func (ctx *Context) Get(key interface{}) interface{} {
	return ctx.ctx.Value(key)
}

// SetValue sets the kv pair.
func (ctx *Context) SetValue(key, val interface{}) {
	if key == nil || val == nil {
		return
	}
	ctx.ctx.SetValue(key, val)
}

// DeleteKey delete the kv pair by key.
func (ctx *Context) DeleteKey(key interface{}) {
	if ctx.ctx==nil || key == nil{
		return
	}
	ctx.ctx.DeleteKey(key)
}


// Payload returns the  payload.
func (ctx *Context) Payload() []byte {
	return ctx.req.Payload
}

// Metadata returns the metadata.
func (ctx *Context) Metadata() map[string]string {
	return ctx.req.Metadata
}

// ServicePath returns the ServicePath.
func (ctx *Context) ServicePath() string {
	return ctx.req.ServicePath
}

// ServiceMethod returns the ServiceMethod.
func (ctx *Context) ServiceMethod() string {
	return ctx.req.ServiceMethod
}

// Bind parses the body data and stores the result to v.
func (ctx *Context) Bind(v interface{}) error {
	req := ctx.req
	if v != nil {
		codec := share.Codecs[req.SerializeType()]
		if codec == nil {
			return fmt.Errorf("can not find codec for %d", req.SerializeType())
		}

		err := codec.Decode(req.Payload, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctx *Context) Write(v interface{}) error {
	req := ctx.req

	if req.IsOneway() { // no need to send response
		return nil
	}

	codec := share.Codecs[req.SerializeType()]
	if codec == nil {
		return fmt.Errorf("can not find codec for %d", req.SerializeType())
	}

	res := req.Clone()
	res.SetMessageType(protocol.Response)

	if v != nil {
		data, err := codec.Encode(v)
		if err != nil {
			return err
		}
		res.Payload = data
	}

	resMetadata := ctx.Get(share.ResMetaDataKey)
	if resMetadata != nil {
		resMetaInCtx := resMetadata.(map[string]string)
		meta := res.Metadata
		if meta == nil {
			res.Metadata = resMetaInCtx
		} else {
			for k, v := range resMetaInCtx {
				if meta[k] == "" {
					meta[k] = v
				}
			}
		}
	}

	if len(res.Payload) > 1024 && req.CompressType() != protocol.None {
		res.SetCompressType(req.CompressType())
	}
	respData := res.EncodeSlicePointer()

	var err error
	if ctx.writeCh != nil {
		ctx.writeCh <- respData
	} else {
		_, err = ctx.conn.Write(*respData)
		protocol.PutData(respData)
	}

	return err
}

func (ctx *Context) WriteError(err error) error {
	req := ctx.req

	if req.IsOneway() { // no need to send response
		return nil
	}

	codec := share.Codecs[req.SerializeType()]
	if codec == nil {
		return fmt.Errorf("can not find codec for %d", req.SerializeType())
	}

	res := req.Clone()
	res.SetMessageType(protocol.Response)

	resMetadata := ctx.Get(share.ResMetaDataKey)
	if resMetadata != nil {
		resMetaInCtx := resMetadata.(map[string]string)
		meta := res.Metadata
		if meta == nil {
			res.Metadata = resMetaInCtx
		} else {
			for k, v := range resMetaInCtx {
				if meta[k] == "" {
					meta[k] = v
				}
			}
		}
	}

	res.SetMessageStatusType(protocol.Error)
	res.Metadata[protocol.ServiceError] = err.Error()

	respData := res.EncodeSlicePointer()
	ctx.conn.Write(*respData)
	protocol.PutData(respData)

	return nil
}
