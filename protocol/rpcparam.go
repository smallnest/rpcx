package protocol

import "github.com/spf13/cast"

type RpcParam struct {
	Values map[string]interface{}
}

func NewRpcParam() (*RpcParam) {
	param := RpcParam{}
	param.Values = make(map[string]interface{})
	return &param
}

func (rp *RpcParam) PutValue(key string, value interface{}) {
	rp.Values[key] = value
}

func (rp *RpcParam) Int64Value(key string) (int64) {
	return cast.ToInt64(rp.Values[key])
}

func (rp *RpcParam) IntValue(key string) (int) {
	return cast.ToInt(rp.Values[key])
}

func (rp *RpcParam) StringValue(key string) (string) {
	return cast.ToString(rp.Values[key])
}

func (rp *RpcParam) Value(key string) (interface{}) {
	return rp.Values[key]
}
