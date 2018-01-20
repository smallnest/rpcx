package server

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"github.com/soheilhy/cmux"
)

func (s *Server) startGateway(network string, ln net.Listener) net.Listener {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		log.Infof("network is not tcp/tcp4/tcp6 so can not start gateway")
		return ln
	}

	m := cmux.New(ln)

	httpLn := m.Match(cmux.HTTP1Fast())
	rpcxLn := m.Match(cmux.Any())

	go s.startHTTP1APIGateway(httpLn)
	go m.Serve()

	return rpcxLn
}

func (s *Server) startHTTP1APIGateway(ln net.Listener) {
	router := httprouter.New()
	router.POST("/*servicePath", s.handleGatewayRequest)
	router.GET("/*servicePath", s.handleGatewayRequest)
	router.PUT("/*servicePath", s.handleGatewayRequest)

	if err := http.Serve(ln, router); err != nil {
		log.Errorf("error in gateway Serve: %s", err)
	}
}

func (s *Server) handleGatewayRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	if r.Header.Get(XServicePath) == "" {
		servicePath := params.ByName("servicePath")
		if strings.HasPrefix(servicePath, "/") {
			servicePath = servicePath[1:]
		}
		r.Header.Set(XServicePath, servicePath)
	}
	servicePath := r.Header.Get(XServicePath)

	wh := w.Header()
	req, err := HTTPRequest2RpcxRequest(r)
	defer protocol.FreeMsg(req)

	//set headers
	wh.Set(XVersion, r.Header.Get(XVersion))
	wh.Set(XMessageID, r.Header.Get(XMessageID))
	wh.Set(XServicePath, servicePath)
	wh.Set(XServiceMethod, r.Header.Get(XServiceMethod))
	wh.Set(XSerializeType, r.Header.Get(XSerializeType))

	if err != nil {
		rh := r.Header
		for k, v := range rh {
			if strings.HasPrefix(k, "X-RPCX-") && len(v) > 0 {
				wh.Set(k, v[0])
			}
		}

		wh.Set(XMessageStatusType, "Error")
		wh.Set(XErrorMessage, err.Error())
		return
	}

	ctx := context.WithValue(context.Background(), StartRequestContextKey, time.Now().UnixNano())
	err = s.auth(ctx, req)
	if err != nil {
		s.Plugins.DoPreWriteResponse(ctx, req)
		wh.Set(XMessageStatusType, "Error")
		wh.Set(XErrorMessage, err.Error())
		w.WriteHeader(401)
		s.Plugins.DoPostWriteResponse(ctx, req, req.Clone(), err)
		return
	}

	resMetadata := make(map[string]string)
	newCtx := context.WithValue(context.WithValue(ctx, share.ReqMetaDataKey, req.Metadata),
		share.ResMetaDataKey, resMetadata)

	res, err := s.handleRequest(newCtx, req)
	defer protocol.FreeMsg(res)

	if err != nil {
		log.Warnf("rpcx: failed to handle gateway request: %v", err)
		wh.Set(XMessageStatusType, "Error")
		wh.Set(XErrorMessage, err.Error())
		w.WriteHeader(500)
		return
	}

	s.Plugins.DoPreWriteResponse(newCtx, req)
	if len(resMetadata) > 0 { //copy meta in context to request
		meta := res.Metadata
		if meta == nil {
			res.Metadata = resMetadata
		} else {
			for k, v := range resMetadata {
				meta[k] = v
			}
		}
	}

	meta := url.Values{}
	for k, v := range res.Metadata {
		meta.Add(k, v)
	}
	wh.Set(XMeta, meta.Encode())
	w.Write(res.Payload)
	s.Plugins.DoPostWriteResponse(newCtx, req, res, err)
}
