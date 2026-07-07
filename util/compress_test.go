package util

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"testing"
)

func TestZip(t *testing.T) {
	s := "%5B%7B%22service%22%3A%22AttrDict%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%2C%7B%22service%22%3A%22BrasInfo%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%5D"
	t.Logf("origin len: %d", len(s))
	data, err := Zip([]byte(s))
	if err != nil {
		t.Fatalf("failed to zip: %v", err)
	}
	t.Logf("zipped len: %d", len(data))
	s2, err := Unzip(data)
	if err != nil {
		t.Fatalf("failed to unzip: %v", err)
	}

	if string(s2) != s {
		t.Fatalf("unzip data is wrong")
	}
}

func BenchmarkZip(b *testing.B) {
	s := "%5B%7B%22service%22%3A%22AttrDict%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%2C%7B%22service%22%3A%22BrasInfo%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%5D"
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			data, err := Zip([]byte(s))
			if err != nil {
				b.Errorf("failed to zip: %v", err)
			}
			_ = data
		}
	})
}

func BenchmarkUnzip(b *testing.B) {
	s := "%5B%7B%22service%22%3A%22AttrDict%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%2C%7B%22service%22%3A%22BrasInfo%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%5D"
	data, err := Zip([]byte(s))
	if err != nil {
		b.Fatalf("failed to zip: %v", err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s2, err := Unzip(data)
			if err != nil {
				b.Errorf("failed to zip: %v", err)
			}
			_ = s2
		}
	})
}

func TestUnzipLimited(t *testing.T) {
	// Build a highly compressible payload that expands well beyond the cap.
	orig := bytes.Repeat([]byte("A"), 1<<20) // 1 MiB of 'A'
	zipped, err := Zip(orig)
	if err != nil {
		t.Fatalf("failed to zip: %v", err)
	}
	if len(zipped) >= len(orig) {
		t.Fatalf("expected zipped data to be much smaller, got %d >= %d", len(zipped), len(orig))
	}

	// No limit: full payload is returned.
	out, err := UnzipLimited(zipped, 0)
	if err != nil {
		t.Fatalf("unexpected error with no limit: %v", err)
	}
	if len(out) != len(orig) {
		t.Fatalf("expected %d bytes, got %d", len(orig), len(out))
	}

	// At the cap: allowed.
	out, err = UnzipLimited(zipped, int64(len(orig)))
	if err != nil {
		t.Fatalf("unexpected error at exact limit: %v", err)
	}
	if len(out) != len(orig) {
		t.Fatalf("expected %d bytes, got %d", len(orig), len(out))
	}

	// Over the cap: rejected.
	_, err = UnzipLimited(zipped, int64(len(orig))-1)
	if !errors.Is(err, ErrDecompressedSizeTooLarge) {
		t.Fatalf("expected ErrDecompressedSizeTooLarge, got %v", err)
	}
}

func oldUnzip(data []byte) ([]byte, error) {
	buf := spBuffer.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		spBuffer.Put(buf)
	}()

	_, err := buf.Write(data)
	if err != nil {
		return nil, err
	}

	gr := spReader.Get().(*gzip.Reader)
	defer func() {
		spReader.Put(gr)
	}()
	err = gr.Reset(buf)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	data, err = io.ReadAll(gr)
	if err != nil {
		return nil, err
	}
	return data, err
}

func BenchmarkUnzip_Old(b *testing.B) {
	s := "%5B%7B%22service%22%3A%22AttrDict%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%2C%7B%22service%22%3A%22BrasInfo%22%2C%22service_address%22%3A%22udp%40127.0.0.1%3A5353%22%7D%5D"
	data, err := Zip([]byte(s))
	if err != nil {
		b.Fatalf("failed to zip: %v", err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s2, err := oldUnzip(data)
			if err != nil {
				b.Errorf("failed to zip: %v", err)
			}
			_ = s2
		}
	})
}
