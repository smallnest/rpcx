package codec

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCodec_JSON(t *testing.T) {
	type Something struct {
		A int
		B string
		C int64
		D float64
		E map[string]string
		F json.Number
	}

	st := Something{
		A: 100,
		B: "hello world",
		C: 123456789,
		D: 1234567.89,
		E: map[string]string{"name": "Jerry"},
		F: "123456789",
	}

	var jsonCodec JSONCodec
	data, err := jsonCodec.Encode(&st)
	if err != nil {
		t.Fatal(err)
	}

	var st2 Something
	err = jsonCodec.Decode(data, &st2)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(st, st2) {
		t.Fatalf("expect %+v, but got %+v", st, st2)
	}
}
