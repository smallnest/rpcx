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

const (
	// ServiceError contains error info of service invocation
	ServiceError = "__rpcx_error__"
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
	// None does not compress.
	None CompressType = iota
	// Gzip uses gzip compression.
	Gzip
)

// SerializeType defines serialization type of payload.
type SerializeType byte

const (
	// SerializeNone uses raw []byte and don't serialize/deserialize
	SerializeNone SerializeType = iota
	// JSON for payload.
	JSON
	// ProtoBuffer for payload.
	ProtoBuffer
	// MsgPack for payload
	MsgPack
)

// Message is the generic type of Request and Response.
type Message struct {
	*Header
	ServicePath   string
	ServiceMethod string
	metaBytes     []byte
	Metadata      map[string]string
	Payload       []byte
}

// NewMessage creates an empty message.
func NewMessage() *Message {
	header := Header([12]byte{})
	header[0] = magicNumber

	return &Message{
		Header: &header,
	}
}

// Header is the first part of Message and has fixed size.
// Format:
//
type Header [12]byte

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
func (h *Header) SetMessageType(mt MessageType) {
	h[2] = h[2] | (byte(mt) << 7)
}

// IsHeartbeat returns whether the message is heartbeat message.
func (h Header) IsHeartbeat() bool {
	return h[2]&0x40 == 0x40
}

// SetHeartbeat sets the heartbeat flag.
func (h *Header) SetHeartbeat(hb bool) {
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
func (h *Header) SetOneway(oneway bool) {
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
func (h *Header) SetCompressType(ct CompressType) {
	h[2] = h[2] | ((byte(ct) << 2) & 0x1C)
}

// MessageStatusType returns the message status type.
func (h Header) MessageStatusType() MessageStatusType {
	return MessageStatusType(h[2] & 0x03)
}

// SetMessageStatusType sets message status type.
func (h *Header) SetMessageStatusType(mt MessageStatusType) {
	h[2] = h[2] | (byte(mt) & 0x03)
}

// SerializeType returns serialization type of payload.
func (h Header) SerializeType() SerializeType {
	return SerializeType((h[3] & 0xF0) >> 4)
}

// SetSerializeType sets the serialization type.
func (h *Header) SetSerializeType(st SerializeType) {
	h[3] = h[3] | (byte(st) << 4)
}

// Seq returns sequence number of messages.
func (h Header) Seq() uint64 {
	return binary.BigEndian.Uint64(h[4:])
}

// SetSeq sets  sequence number.
func (h *Header) SetSeq(seq uint64) {
	binary.BigEndian.PutUint64(h[4:], seq)
}

// Clone clones from an message.
func (m Message) Clone() *Message {
	header := *m.Header
	c := &Message{
		Header:        &header,
		ServicePath:   m.ServicePath,
		ServiceMethod: m.ServiceMethod,
	}
	return c
}

// Encode encodes messages.
func (m Message) Encode() []byte {
	meta := encodeMetadata(m.Metadata)

	spL := len(m.ServicePath)
	spM := len(m.ServiceMethod)

	metaStart := 12 + (4 + spL) + (4 + spM)
	payLoadStart := metaStart + (4 + len(meta))
	l := payLoadStart + (4 + len(m.Payload))

	data := make([]byte, l)
	copy(data, m.Header[:])

	binary.BigEndian.PutUint32(data[12:16], uint32(spL))
	copy(data[16:16+spL], util.StringToSliceByte(m.ServicePath))

	binary.BigEndian.PutUint32(data[16+spL:20+spL], uint32(spM))
	copy(data[20+spL:metaStart], util.StringToSliceByte(m.ServiceMethod))

	binary.BigEndian.PutUint32(data[metaStart:metaStart+4], uint32(len(meta)))
	copy(data[metaStart+4:], meta)

	binary.BigEndian.PutUint32(data[payLoadStart:payLoadStart+4], uint32(len(m.Payload)))
	copy(data[payLoadStart+4:], m.Payload)

	return data
}

// WriteTo writes message to writers.
func (m Message) WriteTo(w io.Writer) error {
	_, err := w.Write(m.Header[:])
	if err != nil {
		return err
	}

	//write servicePath and serviceMethod
	err = binary.Write(w, binary.BigEndian, uint32(len(m.ServicePath)))
	if err != nil {
		return err
	}
	_, err = w.Write(util.StringToSliceByte(m.ServicePath))
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(m.ServiceMethod)))
	if err != nil {
		return err
	}
	_, err = w.Write(util.StringToSliceByte(m.ServiceMethod))
	if err != nil {
		return err
	}

	// write meta
	meta := encodeMetadata(m.Metadata)
	err = binary.Write(w, binary.BigEndian, uint32(len(meta)))
	if err != nil {
		return err
	}
	_, err = w.Write(meta)
	if err != nil {
		return err
	}

	//write payload
	err = binary.Write(w, binary.BigEndian, uint32(len(m.Payload)))
	if err != nil {
		return err
	}

	_, err = w.Write(m.Payload)
	return err
}

// len,string,len,string,......
func encodeMetadata(m map[string]string) []byte {
	if len(m) == 0 {
		return []byte{}
	}
	var buf bytes.Buffer
	var d = make([]byte, 4)
	for k, v := range m {
		binary.BigEndian.PutUint32(d, uint32(len(k)))
		buf.Write(d)
		buf.Write(util.StringToSliceByte(k))
		binary.BigEndian.PutUint32(d, uint32(len(v)))
		buf.Write(d)
		buf.Write(util.StringToSliceByte(v))
	}
	return buf.Bytes()
}

func decodeMetadata(lenData []byte, r io.Reader) ([]byte, map[string]string, error) {
	_, err := io.ReadFull(r, lenData)
	if err != nil {
		return nil, nil, err
	}
	l := binary.BigEndian.Uint32(lenData)
	if l == 0 {
		return nil, nil, nil
	}

	m := make(map[string]string, 10)

	data := make([]byte, l)
	_, err = io.ReadFull(r, data)
	if err != nil {
		return nil, nil, err
	}

	n := uint32(0)
	for n < l {
		// parse one key and value
		// key
		sl := binary.BigEndian.Uint32(data[n : n+4])
		n = n + 4
		if n+sl > l-4 {
			return nil, m, ErrMetaKVMissing
		}
		k := util.SliceByteToString(data[n : n+sl])
		n = n + sl

		// value
		sl = binary.BigEndian.Uint32(data[n : n+4])
		n = n + 4
		if n+sl > l {
			return nil, m, ErrMetaKVMissing
		}
		v := util.SliceByteToString(data[n : n+sl])
		n = n + sl
		m[k] = v
	}

	return data, m, nil
}

// Read reads a message from r.
func Read(r io.Reader) (*Message, error) {
	msg := NewMessage()
	err := msg.Decode(r)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// Decode decodes a message from reader.
func (m *Message) Decode(r io.Reader) error {
	// parse header
	_, err := io.ReadFull(r, m.Header[:])
	if err != nil {
		return err
	}

	// parse servicePath
	lenData := make([]byte, 4)

	_, err = io.ReadFull(r, lenData)
	if err != nil {
		return err
	}
	l := binary.BigEndian.Uint32(lenData)
	sp := make([]byte, l)
	_, err = io.ReadFull(r, sp)
	if err != nil {
		return err
	}
	m.ServicePath = util.SliceByteToString(sp)

	// parse serviceMethod
	_, err = io.ReadFull(r, lenData)
	if err != nil {
		return err
	}
	l = binary.BigEndian.Uint32(lenData)
	sm := make([]byte, l)
	_, err = io.ReadFull(r, sm)
	if err != nil {
		return err
	}
	m.ServiceMethod = util.SliceByteToString(sm)

	// parse meta
	m.metaBytes, m.Metadata, err = decodeMetadata(lenData, r)
	if err != nil {
		return err
	}

	// parse payload
	_, err = io.ReadFull(r, lenData)
	if err != nil {
		return err
	}
	l = binary.BigEndian.Uint32(lenData)
	m.Payload = make([]byte, l)

	_, err = io.ReadFull(r, m.Payload)

	return err
}
