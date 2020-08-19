package server

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/cors"
	"github.com/smallnest/rpcx/v5/protocol"
	"github.com/smallnest/rpcx/v5/share"
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

	ctx := context.WithValue(r.Context(), RemoteConnContextKey, conn)

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

	req := protocol.GetPooledMsg()
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
		s.Plugins.DoPreWriteResponse(ctx, req, nil)
		res.Error = &JSONRPCError{
			Code:    CodeInternalJSONRPCError,
			Message: err.Error(),
		}
		s.Plugins.DoPostWriteResponse(ctx, req, req.Clone(), err)
		return res
	}

	resp, err := s.handleRequest(context.Background(), req)
	if r.ID == nil {
		return nil
	}

	s.Plugins.DoPreWriteResponse(ctx, req, nil)
	if err != nil {
		res.Error = &JSONRPCError{
			Code:    CodeInternalJSONRPCError,
			Message: err.Error(),
		}
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

type CORSOptions struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present in the list, all origins will be allowed.
	// An origin may contain a wildcard (*) to replace 0 or more characters
	// (i.e.: http://*.domain.com). Usage of wildcards implies a small performance penalty.
	// Only one wildcard can be used per origin.
	// Default value is ["*"]
	AllowedOrigins []string
	// AllowOriginFunc is a custom function to validate the origin. It take the origin
	// as argument and returns true if allowed or false otherwise. If this option is
	// set, the content of AllowedOrigins is ignored.
	AllowOriginFunc func(origin string) bool
	// AllowOriginFunc is a custom function to validate the origin. It takes the HTTP Request object and the origin as
	// argument and returns true if allowed or false otherwise. If this option is set, the content of `AllowedOrigins`
	// and `AllowOriginFunc` is ignored.
	AllowOriginRequestFunc func(r *http.Request, origin string) bool
	// AllowedMethods is a list of methods the client is allowed to use with
	// cross-domain requests. Default value is simple methods (HEAD, GET and POST).
	AllowedMethods []string
	// AllowedHeaders is list of non simple headers the client is allowed to use with
	// cross-domain requests.
	// If the special "*" value is present in the list, all headers will be allowed.
	// Default value is [] but "Origin" is always appended to the list.
	AllowedHeaders []string
	// ExposedHeaders indicates which headers are safe to expose to the API of a CORS
	// API specification
	ExposedHeaders []string
	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached
	MaxAge int
	// AllowCredentials indicates whether the request can include user credentials like
	// cookies, HTTP authentication or client side SSL certificates.
	AllowCredentials bool
	// OptionsPassthrough instructs preflight to let other potential next handlers to
	// process the OPTIONS method. Turn this on if your application handles OPTIONS.
	OptionsPassthrough bool
	// Debugging flag adds additional output to debug server side CORS issues
	Debug bool
}

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
//    cors.Options{
//    	AllowedOrigins:   []string{"foo.com"},
//    	AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodDelete},
//    	AllowCredentials: true,
//    }
//
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

		go srv.Serve(ln)
	} else {
		srv.Handler = newServer
		go srv.Serve(ln)
	}

}
