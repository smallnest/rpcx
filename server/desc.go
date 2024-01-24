package server

import (
	"context"
	"reflect"
)

type Handxler func(svc interface{}, ctx context.Context, req interface{}, reply interface{}) error

// ServiceDesc is a detailed description of a service
type ServiceDesc struct {
	ServiceName string
	Methods     []MethodDesc
	HandlerType interface{}
	Metadata    string // 元数据
}

// MethodDesc 方法描述
type MethodDesc struct {
	MethodName  string       // 方法名
	Handler     Handxler     // 方法处理函数
	RequestType reflect.Type // 请求类型
}
