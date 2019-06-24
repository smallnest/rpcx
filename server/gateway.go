package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
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

	rpcxLn := m.Match(rpcxPrefixByteMatcher())

	if !s.DisableJSONRPC {
		jsonrpc2Ln := m.Match(cmux.HTTP1HeaderField("X-JSONRPC-2.0", "true"))
		go s.startJSONRPC2(jsonrpc2Ln)
	}

	if !s.DisableHTTPGateway {
		httpLn := m.Match(cmux.HTTP1Fast())
		go s.startHTTP1APIGateway(httpLn)
	}

	go m.Serve()

	return rpcxLn
}

func rpcxPrefixByteMatcher() cmux.Matcher {
	magic := protocol.MagicNumber()
	return func(r io.Reader) bool {
		buf := make([]byte, 1)
		n, _ := r.Read(buf)
		return n == 1 && buf[0] == magic
	}
}

func (s *Server) startHTTP1APIGateway(ln net.Listener) {
	router := httprouter.New()
	router.POST("/*servicePath", s.handleGatewayRequest)
	router.GET("/*servicePath", s.handleGatewayRequest)
	router.PUT("/*servicePath", s.handleGatewayRequest)

	if s.corsOptions != nil {
		opt := cors.Options(*s.corsOptions)
		c := cors.New(opt)
		mux := c.Handler(router)
		s.mu.Lock()
		s.gatewayHTTPServer = &http.Server{Handler: mux}
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		s.gatewayHTTPServer = &http.Server{Handler: router}
		s.mu.Unlock()
	}

	if err := s.gatewayHTTPServer.Serve(ln); err != nil {
		log.Errorf("error in gateway Serve: %s", err)
	}
}

func (s *Server) closeHTTP1APIGateway(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gatewayHTTPServer != nil {
		return s.gatewayHTTPServer.Shutdown(ctx)
	}

	return nil
}

func (s *Server) handleGatewayRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	ctx := context.WithValue(r.Context(), RemoteConnContextKey, r.RemoteAddr) // notice: It is a string, different with TCP (net.Conn)
	err := s.Plugins.DoPreReadRequest(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

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

	if err == nil && servicePath == "" {
		err = errors.New("empty servicepath")
	} else {
		wh.Set(XServicePath, servicePath)
	}

	if err == nil && r.Header.Get(XServiceMethod) == "" {
		err = errors.New("empty servicemethod")
	} else {
		wh.Set(XServiceMethod, r.Header.Get(XServiceMethod))
	}

	if err == nil && r.Header.Get(XSerializeType) == "" {
		err = errors.New("empty serialized type")
	} else {
		wh.Set(XSerializeType, r.Header.Get(XSerializeType))
	}

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
	err = s.Plugins.DoPostReadRequest(ctx, req, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	ctx = context.WithValue(ctx, StartRequestContextKey, time.Now().UnixNano())
	err = s.auth(ctx, req)
	if err != nil {
		s.Plugins.DoPreWriteResponse(ctx, req, nil)
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

	s.Plugins.DoPreWriteResponse(newCtx, req, nil)
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
