package protocol

import "github.com/spf13/cast"

type RpcResult struct {
	Code    int
	Message string
	Data    interface{}
}

func (rr *RpcResult) Set(Code int, Message string, Data interface{}) {
	rr.Code = Code
	rr.Message = Message
	rr.Data = Data
}

func (rr RpcResult) IntValue() (int) {
	return cast.ToInt(rr.Data)
}

func (rr RpcResult) StringValue() (string) {
	return cast.ToString(rr.Data)
}

func (rr RpcResult) Int64Value() (int64) {
	return cast.ToInt64(rr.Data)
}
