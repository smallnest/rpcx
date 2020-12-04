package codec

import (
	"testing"

	"github.com/smallnest/rpcx/codec/testdata"
)

type ColorGroup struct {
	Id     int      `json:"id" xml:"id,attr" msg:"id"`
	Name   string   `json:"name" xml:"name" msg:"name"`
	Colors []string `json:"colors" xml:"colors" msg:"colors"`
}

var group = ColorGroup{
	Id:     1,
	Name:   "Reds",
	Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
}

func BenchmarkByteCodec_Encode(b *testing.B) {
	var raw = make([]byte, 0, 1024)
	serializer := JSONCodec{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ = serializer.Encode(group)
	}
	b.ReportMetric(float64(len(raw)), "bytes")
}

func BenchmarkPBCodec_Encode(b *testing.B) {
	var raw = make([]byte, 0, 1024)
	serializer := PBCodec{}
	group := testdata.ProtoColorGroup{
		Id:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ = serializer.Encode(&group)
	}
	b.ReportMetric(float64(len(raw)), "bytes")
}

func BenchmarkMsgpackCodec_Encode(b *testing.B) {
	var raw = make([]byte, 0, 1024)
	serializer := MsgpackCodec{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ = serializer.Encode(group)
	}
	b.ReportMetric(float64(len(raw)), "bytes")
}

func BenchmarkThriftCodec_Encode(b *testing.B) {
	var bb = make([]byte, 0, 1024)
	serializer := ThriftCodec{}
	thriftColorGroup := testdata.ThriftColorGroup{
		ID:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb, _ = serializer.Encode(&thriftColorGroup)
	}

	b.ReportMetric(float64(len(bb)), "bytes")
}

func BenchmarkByteCodec_Decode(b *testing.B) {
	serializer := JSONCodec{}
	bytes, _ := serializer.Encode(group)
	result := ColorGroup{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = serializer.Decode(bytes, &result)
	}
}

func BenchmarkPBCodec_Decode(b *testing.B) {
	serializer := PBCodec{}
	bytes, _ := serializer.Encode(group)
	result := ColorGroup{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = serializer.Decode(bytes, &result)
	}
}

func BenchmarkMsgpackCodec_Decode(b *testing.B) {
	serializer := MsgpackCodec{}
	bytes, _ := serializer.Encode(group)
	result := ColorGroup{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = serializer.Decode(bytes, &result)
	}
}

func BenchmarkThriftCodec_Decode(b *testing.B) {
	serializer := ThriftCodec{}
	thriftColorGroup := testdata.ThriftColorGroup{
		ID:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}
	bytes, _ := serializer.Encode(&thriftColorGroup)
	result := testdata.ThriftColorGroup{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = serializer.Decode(bytes, &result)
	}
}
