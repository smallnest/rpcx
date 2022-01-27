package util

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
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

	data, err = ioutil.ReadAll(gr)
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
