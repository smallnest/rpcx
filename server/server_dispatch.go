package server

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
)

// Request dispatch to registered services and functions for Server.
// Extracted from server.go.

func (s *Server) handleRequest(ctx context.Context, req *protocol.Message) (res *protocol.Message, err error) {
	serviceName := req.ServicePath
	methodName := req.ServiceMethod

	res = req.Clone()

	res.SetMessageType(protocol.Response)
	s.serviceMapMu.RLock()
	service := s.serviceMap[serviceName]

	if share.Trace {
		log.Debugf("server get service %+v for an request %+v", service, req)
	}

	s.serviceMapMu.RUnlock()
	if service == nil {
		err = errors.New("rpcx: can't find service " + serviceName)
		return s.handleError(res, err)
	}
	mtype := service.method[methodName]
	if mtype == nil {
		if service.function[methodName] != nil { // check raw functions
			return s.handleRequestForFunction(ctx, req)
		}
		err = errors.New("rpcx: can't find method " + methodName)
		return s.handleError(res, err)
	}

	// get a argv object from object pool
	argv := reflectTypePools.Get(mtype.ArgType)

	codec := share.Codecs[req.SerializeType()]
	if codec == nil {
		reflectTypePools.Put(mtype.ArgType, argv)
		err = fmt.Errorf("can not find codec for %d", req.SerializeType())
		return s.handleError(res, err)
	}

	err = codec.Decode(req.Payload, argv)
	if err != nil {
		reflectTypePools.Put(mtype.ArgType, argv)
		return s.handleError(res, err)
	}

	// and get a reply object from object pool
	replyv := reflectTypePools.Get(mtype.ReplyType)

	argv, err = s.Plugins.DoPreCall(ctx, serviceName, methodName, argv)
	if err != nil {
		// return reply to object pool
		reflectTypePools.Put(mtype.ReplyType, replyv)
		return s.handleError(res, err)
	}

	if mtype.ArgType.Kind() != reflect.Ptr {
		err = service.call(ctx, mtype, reflect.ValueOf(argv).Elem(), reflect.ValueOf(replyv))
	} else {
		err = service.call(ctx, mtype, reflect.ValueOf(argv), reflect.ValueOf(replyv))
	}

	replyv, err1 := s.Plugins.DoPostCall(ctx, serviceName, methodName, argv, replyv, err)
	if err == nil {
		err = err1
	}

	// return argc to object pool
	reflectTypePools.Put(mtype.ArgType, argv)

	if err != nil {
		if replyv != nil {
			data, err := codec.Encode(replyv)
			// return reply to object pool
			reflectTypePools.Put(mtype.ReplyType, replyv)
			if err != nil {
				return s.handleError(res, err)
			}
			res.Payload = data
		}
		return s.handleError(res, err)
	}

	if !req.IsOneway() {
		data, err := codec.Encode(replyv)
		// return reply to object pool
		reflectTypePools.Put(mtype.ReplyType, replyv)
		if err != nil {
			return s.handleError(res, err)
		}
		res.Payload = data
	} else if replyv != nil {
		reflectTypePools.Put(mtype.ReplyType, replyv)
	}

	if share.Trace {
		log.Debugf("server called service %+v for an request %+v", service, req)
	}

	return res, nil
}

func (s *Server) handleRequestForFunction(ctx context.Context, req *protocol.Message) (res *protocol.Message, err error) {
	res = req.Clone()

	res.SetMessageType(protocol.Response)

	serviceName := req.ServicePath
	methodName := req.ServiceMethod
	s.serviceMapMu.RLock()
	service := s.serviceMap[serviceName]
	s.serviceMapMu.RUnlock()
	if service == nil {
		err = errors.New("rpcx: can't find service  for func raw function")
		return s.handleError(res, err)
	}
	mtype := service.function[methodName]
	if mtype == nil {
		err = errors.New("rpcx: can't find method " + methodName)
		return s.handleError(res, err)
	}

	argv := reflectTypePools.Get(mtype.ArgType)

	codec := share.Codecs[req.SerializeType()]
	if codec == nil {
		reflectTypePools.Put(mtype.ArgType, argv)
		err = fmt.Errorf("can not find codec for %d", req.SerializeType())
		return s.handleError(res, err)
	}

	err = codec.Decode(req.Payload, argv)
	if err != nil {
		reflectTypePools.Put(mtype.ArgType, argv)
		return s.handleError(res, err)
	}

	replyv := reflectTypePools.Get(mtype.ReplyType)
	argv, err = s.Plugins.DoPreCall(ctx, serviceName, methodName, argv)
	if err != nil {
		// return reply to object pool
		reflectTypePools.Put(mtype.ReplyType, replyv)
		return s.handleError(res, err)
	}

	if mtype.ArgType.Kind() != reflect.Ptr {
		err = service.callForFunction(ctx, mtype, reflect.ValueOf(argv).Elem(), reflect.ValueOf(replyv))
	} else {
		err = service.callForFunction(ctx, mtype, reflect.ValueOf(argv), reflect.ValueOf(replyv))
	}

	replyv, err1 := s.Plugins.DoPostCall(ctx, serviceName, methodName, argv, replyv, err)
	if err == nil {
		err = err1
	}

	reflectTypePools.Put(mtype.ArgType, argv)
	if err != nil {
		reflectTypePools.Put(mtype.ReplyType, replyv)
		return s.handleError(res, err)
	}

	if !req.IsOneway() {
		data, err := codec.Encode(replyv)
		reflectTypePools.Put(mtype.ReplyType, replyv)
		if err != nil {
			return s.handleError(res, err)
		}
		res.Payload = data
	} else if replyv != nil {
		reflectTypePools.Put(mtype.ReplyType, replyv)
	}

	return res, nil
}

func (s *Server) handleError(res *protocol.Message, err error) (*protocol.Message, error) {
	res.SetMessageStatusType(protocol.Error)
	if res.Metadata == nil {
		res.Metadata = make(map[string]string)
	}

	if s.ServerErrorFunc != nil {
		res.Metadata[protocol.ServiceError] = s.ServerErrorFunc(res, err)
	} else {
		res.Metadata[protocol.ServiceError] = err.Error()
	}

	return res, err
}
