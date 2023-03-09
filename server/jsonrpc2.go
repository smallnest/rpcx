package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/cors"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"github.com/soheilhy/cmux"
)

func (s *Server) jsonrpcHandler(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var req = &jsonrpcRequest{}

	err = json.Unmarshal(data, req)
	if err != nil {
		var res = &jsonrpcRespone{}
		res.Error = &JSONRPCError{
			Code:    CodeParseJSONRPCError,
			Message: err.Error(),
		}

		writeResponse(w, res)
		return
	}
	conn := r.Context().Value(HttpConnContextKey).(net.Conn)

	ctx := share.WithValue(r.Context(), RemoteConnContextKey, conn)

	if req.ID != nil {
		res := s.handleJSONRPCRequest(ctx, req, r.Header)
		writeResponse(w, res)
		return
	}

	// notification
	go s.handleJSONRPCRequest(ctx, req, r.Header)
}

func (s *Server) handleJSONRPCRequest(ctx context.Context, r *jsonrpcRequest, header http.Header) *jsonrpcRespone {
	s.Plugins.DoPreReadRequest(ctx)

	var res = &jsonrpcRespone{}
	res.ID = r.ID

	req := protocol.NewMessage()
	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}

	if r.ID == nil {
		req.SetOneway(true)
	}
	req.SetMessageType(protocol.Request)
	req.SetSerializeType(protocol.JSON)

	lastDot := strings.LastIndex(r.Method, ".")
	if lastDot <= 0 {
		res.Error = &JSONRPCError{
			Code:    CodeMethodNotFound,
			Message: "must contains servicepath and method",
		}
		return res
	}
	req.ServicePath = r.Method[:lastDot]
	req.ServiceMethod = r.Method[lastDot+1:]
	req.Payload = *r.Params

	// meta
	meta := header.Get(XMeta)
	if meta != "" {
		metadata, _ := url.ParseQuery(meta)
		for k, v := range metadata {
			if len(v) > 0 {
				req.Metadata[k] = v[0]
			}
		}
	}

	auth := header.Get("Authorization")
	if auth != "" {
		req.Metadata[share.AuthKey] = auth
	}

	err := s.Plugins.DoPostReadRequest(ctx, req, nil)
	if err != nil {
		res.Error = &JSONRPCError{
			Code:    CodeInternalJSONRPCError,
			Message: err.Error(),
		}
		return res
	}

	err = s.auth(ctx, req)
	if err != nil {
		s.Plugins.DoPreWriteResponse(ctx, req, nil, err)
		res.Error = &JSONRPCError{
			Code:    CodeInternalJSONRPCError,
			Message: err.Error(),
		}
		s.Plugins.DoPostWriteResponse(ctx, req, req.Clone(), err)
		return res
	}

	resp, err := s.handleRequest(ctx, req)
	if r.ID == nil {
		return nil
	}

	s.Plugins.DoPreWriteResponse(ctx, req, nil, err)
	if err != nil {
		res.Error = &JSONRPCError{
			Code:    CodeInternalJSONRPCError,
			Message: err.Error(),
		}
		s.Plugins.DoPostWriteResponse(ctx, req, req.Clone(), err)
		return res
	}

	result := json.RawMessage(resp.Payload)
	res.Result = &result
	s.Plugins.DoPostWriteResponse(ctx, req, req.Clone(), err)
	return res
}

func writeResponse(w http.ResponseWriter, res *jsonrpcRespone) {
	data, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Context-Type", "application/json")
	w.Write(data)
}

type CORSOptions = cors.Options

// AllowAllCORSOptions returns a option that allows access.
func AllowAllCORSOptions() *CORSOptions {
	return &CORSOptions{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
	}
}

// SetCORS sets CORS options.
// for example:
//
//	cors.Options{
//		AllowedOrigins:   []string{"foo.com"},
//		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodDelete},
//		AllowCredentials: true,
//	}
func (s *Server) SetCORS(options *CORSOptions) {
	s.corsOptions = options
}

func (s *Server) startJSONRPC2(ln net.Listener) {
	newServer := http.NewServeMux()
	newServer.HandleFunc("/", s.jsonrpcHandler)

	srv := http.Server{ConnContext: func(ctx context.Context, c net.Conn) context.Context {
		return context.WithValue(ctx, HttpConnContextKey, c)
	}}

	if s.corsOptions != nil {
		opt := cors.Options(*s.corsOptions)
		c := cors.New(opt)
		mux := c.Handler(newServer)
		srv.Handler = mux
	} else {
		srv.Handler = newServer
	}

	s.jsonrpcHTTPServer = &srv
	if err := s.jsonrpcHTTPServer.Serve(ln); !errors.Is(err, cmux.ErrServerClosed) {
		log.Errorf("error in JSONRPC server: %T %s", err, err)
	}
}

func (s *Server) closeJSONRPC2(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jsonrpcHTTPServer != nil {
		return s.jsonrpcHTTPServer.Shutdown(ctx)
	}

	return nil
}
