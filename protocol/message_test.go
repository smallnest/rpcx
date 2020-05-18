package protocol

import (
	"bytes"
	"fmt"
	"testing"
)

func TestMessage(t *testing.T) {
	req := NewMessage()
	req.SetVersion(0)
	req.SetMessageType(Request)
	req.SetHeartbeat(false)
	req.SetOneway(false)
	req.SetCompressType(None)
	req.SetMessageStatusType(Normal)
	req.SetSerializeType(JSON)

	req.SetSeq(1234567890)

	m := make(map[string]string)
	req.ServicePath = "Arith"
	req.ServiceMethod = "Add"
	m["__ID"] = "6ba7b810-9dad-11d1-80b4-00c04fd430c9"
	req.Metadata = m

	payload := `{
		"A": 1,
		"B": 2,
	}
	`
	req.Payload = []byte(payload)

	var buf bytes.Buffer
	_, err := req.WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(buf.Bytes())

	res, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	res.SetMessageType(Response)

	if res.Version() != 0 {
		t.Errorf("expect 0 but got %d", res.Version())
	}

	if res.Seq() != 1234567890 {
		t.Errorf("expect 1234567890 but got %d", res.Seq())
	}

	if res.ServicePath != "Arith" || res.ServiceMethod != "Add" || res.Metadata["__ID"] != "6ba7b810-9dad-11d1-80b4-00c04fd430c9" {
		t.Errorf("got wrong metadata: %v", res.Metadata)
	}

	if string(res.Payload) != payload {
		t.Errorf("got wrong payload: %v", string(res.Payload))
	}
}
