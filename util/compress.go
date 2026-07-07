package util

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"sync"
)

// ErrDecompressedSizeTooLarge is returned when the decompressed data exceeds
// the configured maximum size. It guards against decompression-bomb attacks.
var ErrDecompressedSizeTooLarge = errors.New("decompressed size exceeds the maximum allowed size")

var (
	spWriter sync.Pool
	spReader sync.Pool
	spBuffer sync.Pool
)

func init() {
	spWriter = sync.Pool{New: func() any {
		return gzip.NewWriter(nil)
	}}
	spReader = sync.Pool{New: func() any {
		return new(gzip.Reader)
	}}
	spBuffer = sync.Pool{New: func() any {
		return bytes.NewBuffer(nil)
	}}
}

// Unzip unzips data. If MaxDecompressedSize (see below) is set to a positive
// value, the decompressed size is capped to guard against decompression bombs.
func Unzip(data []byte) ([]byte, error) {
	return UnzipLimited(data, MaxDecompressedSize)
}

// MaxDecompressedSize is the default maximum allowed size (in bytes) of
// decompressed data used by Unzip. A value <= 0 means no limit.
//
// It protects against decompression-bomb attacks where a small compressed
// payload expands to a huge amount of memory. Callers that read from
// untrusted peers should set this to a sane value.
var MaxDecompressedSize int64 = 0

// UnzipLimited unzips data, capping the decompressed size to maxSize bytes.
// A maxSize <= 0 means no limit. It returns ErrDecompressedSizeTooLarge if the
// decompressed data would exceed maxSize.
func UnzipLimited(data []byte, maxSize int64) ([]byte, error) {
	buf := bytes.NewBuffer(data)

	gr := spReader.Get().(*gzip.Reader)
	defer func() {
		spReader.Put(gr)
	}()
	err := gr.Reset(buf)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	if maxSize <= 0 {
		return io.ReadAll(gr)
	}

	// Read at most maxSize+1 bytes so we can detect an overflow without
	// allocating the entire oversized payload.
	limited := io.LimitReader(gr, maxSize+1)
	out, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > maxSize {
		return nil, ErrDecompressedSizeTooLarge
	}
	return out, nil
}

// Zip zips data.
func Zip(data []byte) ([]byte, error) {
	buf := spBuffer.Get().(*bytes.Buffer)
	w := spWriter.Get().(*gzip.Writer)
	w.Reset(buf)

	defer func() {
		buf.Reset()
		spBuffer.Put(buf)
		w.Close()
		spWriter.Put(w)
	}()
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Flush()
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	dec := buf.Bytes()
	return dec, nil
}
