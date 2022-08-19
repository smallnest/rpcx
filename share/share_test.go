package share

import (
	"testing"

	"github.com/smallnest/rpcx/protocol"
	"github.com/stretchr/testify/assert"
)

type MockCodec struct {}

func (codec MockCodec) Encode(i interface{}) ([]byte, error) {
	return nil, nil
}

func (codec MockCodec) Decode(data []byte, i interface{}) error {
	return nil
}

func TestShare(t *testing.T) {
	registeredCodecNum := len(Codecs)
	codec := MockCodec{}

	mockCodecType := 127
	RegisterCodec(protocol.SerializeType(mockCodecType), codec)
	assert.Equal(t, registeredCodecNum + 1, len(Codecs))
}
