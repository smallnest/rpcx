package codec

import (
	"encoding/json"
	"fmt"

	proto "github.com/gogo/protobuf/proto"
	"github.com/vmihailenco/msgpack"
)

// Codec defines the interface that decode/encode payload.
type Codec interface {
	Encode(i interface{}) ([]byte, error)
	Decode(data []byte, i interface{}) error
}

// ByteCodec uses raw slice pf bytes and don't encode/decode.
type ByteCodec struct{}

// Encode returns raw slice of bytes.
func (c ByteCodec) Encode(i interface{}) ([]byte, error) {
	if data, ok := i.([]byte); ok {
		return data, nil
	}

	return nil, fmt.Errorf("%T is not a []byte", i)
}

// Decode returns raw slice of bytes.
func (c ByteCodec) Decode(data []byte, i interface{}) error {
	i = &data
	return nil
}

// JSONCodec uses json marshaler and unmarshaler.
type JSONCodec struct{}

// Encode encodes an object into slice of bytes.
func (c JSONCodec) Encode(i interface{}) ([]byte, error) {
	return json.Marshal(i)
}

// Decode decodes an object from slice of bytes.
func (c JSONCodec) Decode(data []byte, i interface{}) error {
	return json.Unmarshal(data, i)
}

// PBCodec uses protobuf marshaler and unmarshaler.
type PBCodec struct{}

// Encode encodes an object into slice of bytes.
func (c PBCodec) Encode(i interface{}) ([]byte, error) {
	if m, ok := i.(proto.Marshaler); ok {
		return m.Marshal()
	}

	return nil, fmt.Errorf("%T is not a proto.Marshaler", i)
}

// Decode decodes an object from slice of bytes.
func (c PBCodec) Decode(data []byte, i interface{}) error {
	if m, ok := i.(proto.Unmarshaler); ok {
		return m.Unmarshal(data)
	}

	return fmt.Errorf("%T is not a proto.Unmarshaler", i)
}

// MsgpackCodec uses messagepack marshaler and unmarshaler.
type MsgpackCodec struct{}

// Encode encodes an object into slice of bytes.
func (c MsgpackCodec) Encode(i interface{}) ([]byte, error) {
	return msgpack.Marshal(i)
}

// Decode decodes an object from slice of bytes.
func (c MsgpackCodec) Decode(data []byte, i interface{}) error {
	return msgpack.Unmarshal(data, i)
}
