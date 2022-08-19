package protocol

import (
	"reflect"
	"testing"

	"github.com/smallnest/rpcx/codec"
	"github.com/smallnest/rpcx/protocol/testdata"
)

func newBenchmarkMessage() *testdata.BenchmarkMessage {
	var theAnswer = "Answer to the Ultimate Question of Life, the Universe, and Everything"
	var args testdata.BenchmarkMessage

	v := reflect.ValueOf(&args).Elem()

	for k := 0; k < v.NumField(); k++ {
		field := v.Field(k)
		fieldName := v.Type().Field(k).Name
		// filter unexported fields
		if fieldName[0] >= 97 && fieldName[0] <= 122 {
			continue
		}

		switch field.Kind() {
		case reflect.Int, reflect.Int32, reflect.Int64:
			field.SetInt(31415926)
		case reflect.Bool:
			field.SetBool(true)
		case reflect.String:
			field.SetString(theAnswer)
		}
	}

	return &args
}

func BenchmarkGzipCompressor_Zip(b *testing.B) {
	compressor := GzipCompressor{}
	serializer := codec.PBCodec{}
	raw, _ := serializer.Encode(newBenchmarkMessage())
	zipped := make([]byte, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zipped, _ = compressor.Zip(raw)
	}
	b.ReportMetric(float64(len(zipped)), "bytes")
}

func BenchmarkRawDataCompressor_Zip(b *testing.B) {
	compressor := RawDataCompressor{}
	serializer := codec.PBCodec{}
	raw, _ := serializer.Encode(newBenchmarkMessage())
	zipped := make([]byte, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zipped, _ = compressor.Zip(raw)
	}
	b.ReportMetric(float64(len(zipped)), "bytes")
}

func BenchmarkSnappyCompressor_Zip(b *testing.B) {
	compressor := SnappyCompressor{}
	serializer := codec.PBCodec{}
	raw, _ := serializer.Encode(newBenchmarkMessage())
	zipped := make([]byte, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zipped, _ = compressor.Zip(raw)
	}
	b.ReportMetric(float64(len(zipped)), "bytes")
}

func BenchmarkGzipCompressor_Unzip(b *testing.B) {
	compressor := GzipCompressor{}
	serializer := codec.PBCodec{}
	raw, _ := serializer.Encode(newBenchmarkMessage())
	zipped, _ := compressor.Zip(raw)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ = compressor.Unzip(zipped)
	}
	b.ReportMetric(float64(len(raw)), "bytes")
}

func BenchmarkRawDataCompressor_Unzip(b *testing.B) {
	compressor := RawDataCompressor{}
	serializer := codec.PBCodec{}
	raw, _ := serializer.Encode(newBenchmarkMessage())
	zipped, _ := compressor.Zip(raw)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ = compressor.Unzip(zipped)
	}
	b.ReportMetric(float64(len(raw)), "bytes")
}

func BenchmarkSnappyCompressor_Unzip(b *testing.B) {
	compressor := SnappyCompressor{}
	serializer := codec.PBCodec{}
	raw, _ := serializer.Encode(newBenchmarkMessage())
	zipped, _ := compressor.Zip(raw)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ = compressor.Unzip(zipped)
	}
	b.ReportMetric(float64(len(raw)), "bytes")
}
