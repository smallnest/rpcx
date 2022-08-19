package protocol

import (
	"bytes"
	"io/ioutil"

	"github.com/golang/snappy"
	"github.com/smallnest/rpcx/util"
)

// Compressor defines a common compression interface.
type Compressor interface {
	Zip([]byte) ([]byte, error)
	Unzip([]byte) ([]byte, error)
}

// GzipCompressor implements gzip compressor.
type GzipCompressor struct {
}

func (c GzipCompressor) Zip(data []byte) ([]byte, error) {
	return util.Zip(data)
}

func (c GzipCompressor) Unzip(data []byte) ([]byte, error) {
	return util.Unzip(data)
}

type RawDataCompressor struct {
}

func (c RawDataCompressor) Zip(data []byte) ([]byte, error) {
	return data, nil
}

func (c RawDataCompressor) Unzip(data []byte) ([]byte, error) {
	return data, nil
}

// SnappyCompressor implements snappy compressor
type SnappyCompressor struct {
}

func (c *SnappyCompressor) Zip(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	var buffer bytes.Buffer
	writer := snappy.NewBufferedWriter(&buffer)
	_, err := writer.Write(data)
	if err != nil {
		writer.Close()
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func (c *SnappyCompressor) Unzip(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	reader := snappy.NewReader(bytes.NewReader(data))
	out, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return out, err
}
