package server

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
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

// HTTPRequest2RpcxRequest converts a http request to a rpcx request.
func HTTPRequest2RpcxRequest(r *http.Request) (*protocol.Message, error) {
	req := protocol.GetPooledMsg()
	req.SetMessageType(protocol.Request)

	h := r.Header
	seq := h.Get(XMessageID)
	if seq != "" {
		id, err := strconv.ParseUint(seq, 10, 64)
		if err != nil {
			return nil, err
		}
		req.SetSeq(id)
	}

	heartbeat := h.Get(XHeartbeat)
	if heartbeat != "" {
		req.SetHeartbeat(true)
	}

	oneway := h.Get(XOneway)
	if oneway != "" {
		req.SetOneway(true)
	}

	st := h.Get(XSerializeType)
	if st != "" {
		rst, err := strconv.Atoi(st)
		if err != nil {
			return nil, err
		}
		req.SetSerializeType(protocol.SerializeType(rst))
	}

	meta := h.Get(XMeta)
	if meta != "" {
		metadata, err := url.ParseQuery(meta)
		if err != nil {
			return nil, err
		}
		mm := make(map[string]string)
		for k, v := range metadata {
			if len(v) > 0 {
				mm[k] = v[0]
			}
		}
		req.Metadata = mm
	}

	auth := h.Get("Authorization")
	if auth != "" {
		if req.Metadata == nil {
			req.Metadata = make(map[string]string)
		}
		req.Metadata[share.AuthKey] = auth
	}

	sp := h.Get(XServicePath)
	if sp != "" {
		req.ServicePath = sp
	}

	sm := h.Get(XServiceMethod)
	if sm != "" {
		req.ServiceMethod = sm
	}

	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	req.Payload = payload

	return req, nil
}

// func RpcxResponse2HttpResponse(res *protocol.Message) (url.Values, []byte, error) {
// 	m := make(url.Values)
// 	m.Set(XVersion, strconv.Itoa(int(res.Version())))
// 	if res.IsHeartbeat() {
// 		m.Set(XHeartbeat, "true")
// 	}
// 	if res.IsOneway() {
// 		m.Set(XOneway, "true")
// 	}
// 	if res.MessageStatusType() == protocol.Error {
// 		m.Set(XMessageStatusType, "Error")
// 	} else {
// 		m.Set(XMessageStatusType, "Normal")
// 	}

// 	if res.CompressType() == protocol.Gzip {
// 		m.Set("Content-Encoding", "gzip")
// 	}

// 	m.Set(XSerializeType, strconv.Itoa(int(res.SerializeType())))
// 	m.Set(XMessageID, strconv.FormatUint(res.Seq(), 10))
// 	m.Set(XServicePath, res.ServicePath)
// 	m.Set(XServiceMethod, res.ServiceMethod)

// 	return m, res.Payload, nil
// }
