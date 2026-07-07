package protocol

import (
	"bytes"
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

// TestDecodeDecompressBomb verifies that Decode caps the decompressed payload
// size when MaxDecompressedLength is set, guarding against decompression-bomb
// attacks (see issue #942).
func TestDecodeDecompressBomb(t *testing.T) {
	req := NewMessage()
	req.SetVersion(0)
	req.SetMessageType(Request)
	req.SetCompressType(Gzip)
	req.SetSerializeType(SerializeNone)
	req.ServicePath = "Arith"
	req.ServiceMethod = "Add"

	// A small compressed payload that expands to 1 MiB.
	req.Payload = bytes.Repeat([]byte("A"), 1<<20)

	var buf bytes.Buffer
	if _, err := req.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	wire := buf.Bytes()

	// With the cap set below the decompressed size, Decode must reject it.
	old := MaxDecompressedLength
	MaxDecompressedLength = 1024
	defer func() { MaxDecompressedLength = old }()

	if _, err := Read(bytes.NewReader(wire)); err == nil {
		t.Fatal("expected Decode to reject the decompression bomb, got nil error")
	}

	// With no cap, the same message decodes fine.
	MaxDecompressedLength = 0
	res, err := Read(bytes.NewReader(wire))
	if err != nil {
		t.Fatalf("unexpected error without cap: %v", err)
	}
	if len(res.Payload) != 1<<20 {
		t.Fatalf("expected 1 MiB payload, got %d bytes", len(res.Payload))
	}
}
