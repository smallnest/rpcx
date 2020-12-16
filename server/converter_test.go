package server

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/smallnest/rpcx/codec"
	"github.com/smallnest/rpcx/share"
	"github.com/stretchr/testify/assert"
)

func TestHTTPRequest2RpcxRequest(t *testing.T) {

	cc := &codec.MsgpackCodec{}

	args := &Args{
		A: 10,
		B: 20,
	}

	data, _ := cc.Encode(args)

	req, err := http.NewRequest("POST", "http://127.0.0.1:8972/", bytes.NewReader(data))
	if err != nil {
		t.Fatal("failed to create request: ", err)
		return
	}

	h := req.Header
	h.Set(XMessageID, "10000")
	h.Set(XHeartbeat, "0")
	h.Set(XOneway, "0")
	h.Set(XSerializeType, "3")
	h.Set(XMeta, "Meta")
	h.Set("Authorization", "Authorization")
	h.Set(XServicePath, "ProxyServer")
	h.Set(XServiceMethod, "GetAdData")

	rpcxReq, err := HTTPRequest2RpcxRequest(req)
	if err != nil {
		t.Fatal("HTTPRequest2RpcxRequest() error")
	}

	assert.NotNil(t, rpcxReq.Metadata)

	assert.Equal(t, h.Get("Authorization"), rpcxReq.Metadata[share.AuthKey])

	assert.Equal(t, h.Get(XServicePath), rpcxReq.ServicePath)

	assert.Equal(t, h.Get(XServiceMethod), rpcxReq.ServiceMethod)
}
