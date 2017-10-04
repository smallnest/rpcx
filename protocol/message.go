package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/smallnest/rpcx/util"
)

const (
	magicNumber byte = 0x08
)

var (
	lineSeparator = []byte("\r\n")
)

var (
	// ErrMetaKVMissing some keys or values are mssing.
	ErrMetaKVMissing = errors.New("wrong metadata lines. some keys or values are missing")
)

// MessageType is message type of requests and resposnes.
type MessageType byte

const (
	// Request is message type of request
	Request MessageType = iota
	// Response is message type of response
	Response
)

// MessageStatusType is status of messages.
type MessageStatusType byte

const (
	// Normal is normal requests and responses.
	Normal MessageStatusType = iota
	// Error indicates some errors occur.
	Error
)

// CompressType defines decompression type.
type CompressType byte

const (
	// None does not use compression.
	None CompressType = iota
	// Gzip uses gzip compression.
	Gzip
)

// SerializeType defines serialization type of payload.
type SerializeType byte

const (
	// JSON for payload.
	JSON SerializeType = iota
	// ProtoBuffer for payload.
	ProtoBuffer
	// MsgPack for payload
	MsgPack
)

// Message is the generic type of Request and Response.
type Message struct {
	*Header
	Metadata map[string]string
	Payload  []byte
}

// NewMessage creates an empty message.
func NewMessage() *Message {
	header := Header([8]byte{})
	return &Message{
		Header:   &header,
		Metadata: make(map[string]string),
	}
}

// Header is the first part of Message and has fixed size.
// Format:
//
type Header [8]byte

// CheckMagicNumber checks whether header starts rpcx magic number.
func (h Header) CheckMagicNumber() bool {
	return h[0] == magicNumber
}

// Version returns version of rpcx protocol.
func (h Header) Version() byte {
	return h[1]
}

// SetVersion sets version for this header.
func (h *Header) SetVersion(v byte) {
	h[1] = v
}

// MessageType returns the message type.
func (h Header) MessageType() MessageType {
	return MessageType(h[2] & 0x80)
}

// SetMessageType sets message type.
func (h Header) SetMessageType(mt MessageType) {
	h[2] = h[2] | (byte(mt) << 7)
}

// IsHeartbeat returns whether the message is heartbeat message.
func (h Header) IsHeartbeat() bool {
	return h[2]&0x40 == 0x40
}

// SetHeartbeat sets the heartbeat flag.
func (h Header) SetHeartbeat(hb bool) {
	if hb {
		h[2] = h[2] | 0x40
	} else {
		h[2] = h[2] &^ 0x40
	}
}

// IsOneway returns whether the message is one-way message.
// If true, server won't send responses.
func (h Header) IsOneway() bool {
	return h[2]&0x20 == 0x20
}

// SetOneway sets the oneway flag.
func (h Header) SetOneway(oneway bool) {
	if oneway {
		h[2] = h[2] | 0x40
	} else {
		h[2] = h[2] &^ 0x40
	}
}

// CompressType returns compression type of messages.
func (h Header) CompressType() CompressType {
	return CompressType((h[2] & 0x1C) >> 2)
}

// SetCompressType sets the compression type.
func (h Header) SetCompressType(ct CompressType) {
	h[2] = h[2] | (byte(ct) << 2)
}

// SerializeType returns serialization type of payload.
func (h Header) SerializeType() SerializeType {
	return SerializeType((h[3] & 0xF0) >> 4)
}

// SetSerializeType sets the serialization type.
func (h Header) SetSerializeType(st SerializeType) {
	h[3] = h[3] | (byte(st) << 4)
}

// Seq returns sequence number of messages.
func (h Header) Seq() uint64 {
	return binary.BigEndian.Uint64(h[4:])
}

// SeSeq sets  sequence number.
func (h Header) SeSeq(seq uint64) {
	binary.BigEndian.PutUint64(h[4:], seq)
}

// Encode encodes messages.
func (m Message) Encode() []byte {
	meta := encodeMetadata(m.Metadata)

	l := 8 + (4 + len(meta)) + (4 + len(m.Payload))

	data := make([]byte, l)
	copy(data, m.Header[:])
	binary.BigEndian.PutUint32(data[8:12], uint32(len(meta)))
	copy(data[12:], meta)
	binary.BigEndian.PutUint32(data[12+len(meta):], uint32(len(m.Payload)))
	copy(data[16+len(meta):], m.Payload)

	return data
}

func encodeMetadata(m map[string]string) []byte {
	var buf bytes.Buffer
	for k, v := range m {
		buf.WriteString(k)
		buf.Write(lineSeparator)
		buf.WriteString(v)
		buf.Write(lineSeparator)
	}

	return buf.Bytes()
}

func decodeMetadata(lenData []byte, r io.Reader) (map[string]string, error) {
	_, err := io.ReadFull(r, lenData)
	if err != nil {
		return nil, err
	}
	l := binary.BigEndian.Uint32(lenData)
	m := make(map[string]string)
	if l == 0 {
		return m, nil
	}

	data := make([]byte, l)
	_, err = io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}

	meta := bytes.Split(data, lineSeparator)
	if len(meta)%2 != 0 {
		return nil, ErrMetaKVMissing
	}

	for i := 0; i < len(meta); i = i + 2 {
		m[util.SliceByteToString(meta[i])] = util.SliceByteToString(meta[i+1])
	}
	return m, nil
}

func readMessage(r io.Reader) (*Message, error) {
	msg := NewMessage()
	_, err := io.ReadFull(r, msg.Header[:])
	if err != nil {
		return nil, err
	}

	lenData := make([]byte, 4)
	msg.Metadata, err = decodeMetadata(lenData, r)
	if err != nil {
		return nil, err
	}

	_, err = io.ReadFull(r, lenData)
	if err != nil {
		return nil, err
	}
	l := binary.BigEndian.Uint32(lenData)

	msg.Payload = make([]byte, l)

	_, err = io.ReadFull(r, msg.Payload)

	return msg, err
}
