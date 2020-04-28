package server

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	rerrors "github.com/smallnest/rpcx/v5/errors"
	"github.com/smallnest/rpcx/v5/log"
)

// Precompute the reflect type for error. Can't use error directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

// Precompute the reflect type for context.
var typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

type methodType struct {
	sync.Mutex // protects counters
	method     reflect.Method
	ArgType    reflect.Type
	ReplyType  reflect.Type
	// numCalls   uint
}

type functionType struct {
	sync.Mutex // protects counters
	fn         reflect.Value
	ArgType    reflect.Type
	ReplyType  reflect.Type
}

type service struct {
	name     string                   // name of service
	rcvr     reflect.Value            // receiver of methods for the service
	typ      reflect.Type             // type of the receiver
	method   map[string]*methodType   // registered methods
	function map[string]*functionType // registered functions
}

func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- three arguments, the first is of context.Context, both of exported type for three arguments
//	- the third argument is a pointer
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.
func (s *Server) Register(rcvr interface{}, metadata string) error {
	sname, err := s.register(rcvr, "", false)
	if err != nil {
		return err
	}
	return s.Plugins.DoRegister(sname, rcvr, metadata)
}

// RegisterName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.
func (s *Server) RegisterName(name string, rcvr interface{}, metadata string) error {
	s.Plugins.DoRegister(name, rcvr, metadata)
	_, err := s.register(rcvr, name, true)
	return err
}

// RegisterFunction publishes a function that satisfy the following conditions:
//	- three arguments, the first is of context.Context, both of exported type for three arguments
//	- the third argument is a pointer
//	- one return value, of type error
// The client accesses function using a string of the form "servicePath.Method".
func (s *Server) RegisterFunction(servicePath string, fn interface{}, metadata string) error {
	fname, err := s.registerFunction(servicePath, fn, "", false)
	if err != nil {
		return err
	}

	return s.Plugins.DoRegisterFunction(servicePath, fname, fn, metadata)
}

// RegisterFunctionName is like RegisterFunction but uses the provided name for the function
// instead of the function's concrete type.
func (s *Server) RegisterFunctionName(servicePath string, name string, fn interface{}, metadata string) error {
	s.Plugins.DoRegisterFunction(servicePath, name, fn, metadata)
	_, err := s.registerFunction(servicePath, fn, name, true)
	return err
}

func (s *Server) register(rcvr interface{}, name string, useName bool) (string, error) {
	s.serviceMapMu.Lock()
	defer s.serviceMapMu.Unlock()

	service := new(service)
	service.typ = reflect.TypeOf(rcvr)
	service.rcvr = reflect.ValueOf(rcvr)
	sname := reflect.Indirect(service.rcvr).Type().Name() // Type
	if useName {
		sname = name
	}
	if sname == "" {
		errorStr := "rpcx.Register: no service name for type " + service.typ.String()
		log.Error(errorStr)
		return sname, errors.New(errorStr)
	}
	if !useName && !isExported(sname) {
		errorStr := "rpcx.Register: type " + sname + " is not exported"
		log.Error(errorStr)
		return sname, errors.New(errorStr)
	}
	service.name = sname

	// Install the methods
	service.method = suitableMethods(service.typ, true)

	if len(service.method) == 0 {
		var errorStr string

		// To help the user, see if a pointer receiver would work.
		method := suitableMethods(reflect.PtrTo(service.typ), false)
		if len(method) != 0 {
			errorStr = "rpcx.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			errorStr = "rpcx.Register: type " + sname + " has no exported methods of suitable type"
		}
		log.Error(errorStr)
		return sname, errors.New(errorStr)
	}
	s.serviceMap[service.name] = service
	return sname, nil
}

func (s *Server) registerFunction(servicePath string, fn interface{}, name string, useName bool) (string, error) {
	s.serviceMapMu.Lock()
	defer s.serviceMapMu.Unlock()

	ss := s.serviceMap[servicePath]
	if ss == nil {
		ss = new(service)
		ss.name = servicePath
		ss.function = make(map[string]*functionType)
	}

	f, ok := fn.(reflect.Value)
	if !ok {
		f = reflect.ValueOf(fn)
	}
	if f.Kind() != reflect.Func {
		return "", errors.New("function must be func or bound method")
	}

	fname := runtime.FuncForPC(reflect.Indirect(f).Pointer()).Name()
	if fname != "" {
		i := strings.LastIndex(fname, ".")
		if i >= 0 {
			fname = fname[i+1:]
		}
	}
	if useName {
		fname = name
	}
	if fname == "" {
		errorStr := "rpcx.registerFunction: no func name for type " + f.Type().String()
		log.Error(errorStr)
		return fname, errors.New(errorStr)
	}

	t := f.Type()
	if t.NumIn() != 3 {
		return fname, fmt.Errorf("rpcx.registerFunction: has wrong number of ins: %s", f.Type().String())
	}
	if t.NumOut() != 1 {
		return fname, fmt.Errorf("rpcx.registerFunction: has wrong number of outs: %s", f.Type().String())
	}

	// First arg must be context.Context
	ctxType := t.In(0)
	if !ctxType.Implements(typeOfContext) {
		return fname, fmt.Errorf("function %s must use context as  the first parameter", f.Type().String())
	}

	argType := t.In(1)
	if !isExportedOrBuiltinType(argType) {
		return fname, fmt.Errorf("function %s parameter type not exported: %v", f.Type().String(), argType)
	}

	replyType := t.In(2)
	if replyType.Kind() != reflect.Ptr {
		return fname, fmt.Errorf("function %s reply type not a pointer: %s", f.Type().String(), replyType)
	}
	if !isExportedOrBuiltinType(replyType) {
		return fname, fmt.Errorf("function %s reply type not exported: %v", f.Type().String(), replyType)
	}

	// The return type of the method must be error.
	if returnType := t.Out(0); returnType != typeOfError {
		return fname, fmt.Errorf("function %s returns %s, not error", f.Type().String(), returnType.String())
	}

	// Install the methods
	ss.function[fname] = &functionType{fn: f, ArgType: argType, ReplyType: replyType}
	s.serviceMap[servicePath] = ss

	argsReplyPools.Init(argType)
	argsReplyPools.Init(replyType)
	return fname, nil
}

// suitableMethods returns suitable Rpc methods of typ, it will report
// error using log if reportErr is true.
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs four ins: receiver, context.Context, *args, *reply.
		if mtype.NumIn() != 4 {
			if reportErr {
				log.Debug("method ", mname, " has wrong number of ins:", mtype.NumIn())
			}
			continue
		}
		// First arg must be context.Context
		ctxType := mtype.In(1)
		if !ctxType.Implements(typeOfContext) {
			if reportErr {
				log.Debug("method ", mname, " must use context.Context as the first parameter")
			}
			continue
		}

		// Second arg need not be a pointer.
		argType := mtype.In(2)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				log.Info(mname, " parameter type not exported: ", argType)
			}
			continue
		}
		// Third arg must be a pointer.
		replyType := mtype.In(3)
		if replyType.Kind() != reflect.Ptr {
			if reportErr {
				log.Info("method", mname, " reply type not a pointer:", replyType)
			}
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			if reportErr {
				log.Info("method", mname, " reply type not exported:", replyType)
			}
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {
			if reportErr {
				log.Info("method", mname, " has wrong number of outs:", mtype.NumOut())
			}
			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			if reportErr {
				log.Info("method", mname, " returns ", returnType.String(), " not error")
			}
			continue
		}
		methods[mname] = &methodType{method: method, ArgType: argType, ReplyType: replyType}

		argsReplyPools.Init(argType)
		argsReplyPools.Init(replyType)
	}
	return methods
}

// UnregisterAll unregisters all services.
// You can call this method when you want to shutdown/upgrade this node.
func (s *Server) UnregisterAll() error {
	var es []error
	for k := range s.serviceMap {
		err := s.Plugins.DoUnregister(k)
		if err != nil {
			es = append(es, err)
		}
	}

	if len(es) > 0 {
		return rerrors.NewMultiError(es)
	}
	return nil
}

func (s *service) call(ctx context.Context, mtype *methodType, argv, replyv reflect.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var buf = make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]

			err = fmt.Errorf("[service internal error]: %v, method: %s, argv: %+v",
				r, mtype.method.Name, argv.Interface())

			err2 := fmt.Errorf("[service internal error]: %v, method: %s, argv: %+v, stack: %s",
				r, mtype.method.Name, argv.Interface(), buf)
			log.Handle(err2)
		}
	}()

	function := mtype.method.Func
	// Invoke the method, providing a new value for the reply.
	returnValues := function.Call([]reflect.Value{s.rcvr, reflect.ValueOf(ctx), argv, replyv})
	// The return value for the method is an error.
	errInter := returnValues[0].Interface()
	if errInter != nil {
		return errInter.(error)
	}

	return nil
}

func (s *service) callForFunction(ctx context.Context, ft *functionType, argv, replyv reflect.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			//log.Errorf("failed to invoke service: %v, stacks: %s", r, string(debug.Stack()))
			err = fmt.Errorf("[service internal error]: %v, function: %s, argv: %+v",
				r, runtime.FuncForPC(ft.fn.Pointer()), argv.Interface())
			log.Handle(err)
		}
	}()

	// Invoke the function, providing a new value for the reply.
	returnValues := ft.fn.Call([]reflect.Value{reflect.ValueOf(ctx), argv, replyv})
	// The return value for the method is an error.
	errInter := returnValues[0].Interface()
	if errInter != nil {
		return errInter.(error)
	}

	return nil
}
