package protocol

import (
	"bytes"
	"io"

	"github.com/golang/snappy"
	"github.com/smallnest/rpcx/util"
)

// Compressor defines a common compression interface.
type Compressor interface {
	Zip([]byte) ([]byte, error)
	Unzip([]byte) ([]byte, error)
}

// LimitedUnzipper is an optional interface a Compressor can implement to cap
// the size of decompressed data, guarding against decompression-bomb attacks.
// If a compressor implements it, Message.Decode uses it when
// MaxDecompressedSize is set. maxSize <= 0 means no limit.
type LimitedUnzipper interface {
	UnzipLimited(data []byte, maxSize int64) ([]byte, error)
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

func (c GzipCompressor) UnzipLimited(data []byte, maxSize int64) ([]byte, error) {
	return util.UnzipLimited(data, maxSize)
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
	return c.UnzipLimited(data, 0)
}

func (c *SnappyCompressor) UnzipLimited(data []byte, maxSize int64) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	reader := snappy.NewReader(bytes.NewReader(data))
	if maxSize <= 0 {
		return io.ReadAll(reader)
	}

	limited := io.LimitReader(reader, maxSize+1)
	out, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > maxSize {
		return nil, util.ErrDecompressedSizeTooLarge
	}
	return out, nil
}
