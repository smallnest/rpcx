package server

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/smallnest/rpcx/protocol"
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

	if req.ID != nil {
		res := s.handleJSONRPCRequest(req)
		writeResponse(w, res)
		return
	}

	// notification
	go s.handleJSONRPCRequest(req)
}

func (s *Server) handleJSONRPCRequest(r *jsonrpcRequest) *jsonrpcRespone {
	var res = &jsonrpcRespone{}
	res.ID = r.ID

	req := protocol.GetPooledMsg()

	if r.ID == nil {
		req.SetOneway(true)
	}
	req.SetMessageType(protocol.Request)
	req.SetSerializeType(protocol.JSON)

	pathAndMethod := strings.SplitN(r.Method, ".", 2)
	if len(pathAndMethod) != 2 {
		res.Error = &JSONRPCError{
			Code:    CodeMethodNotFound,
			Message: "must contains servicepath and method",
		}
		return res
	}
	req.ServicePath = pathAndMethod[0]
	req.ServiceMethod = pathAndMethod[1]
	req.Payload = *r.Params

	resp, err := s.handleRequest(context.Background(), req)
	if r.ID == nil {
		return nil
	}

	if err != nil {
		res.Error = &JSONRPCError{
			Code:    CodeInternalJSONRPCError,
			Message: err.Error(),
		}
		return res
	}

	result := json.RawMessage(resp.Payload)
	res.Result = &result

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

func (s *Server) startJSONRPC2(ln net.Listener) {
	newServer := http.NewServeMux()
	newServer.HandleFunc("/", s.jsonrpcHandler)
	go http.Serve(ln, newServer)
}
