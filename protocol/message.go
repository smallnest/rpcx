package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/smallnest/rpcx/util"
	"github.com/valyala/bytebufferpool"
)

var bufferPool = util.NewLimitedPool(512, 4096)

// Compressors are compressors supported by rpcx. You can add customized compressor in Compressors.
var Compressors = map[CompressType]Compressor{
	None: &RawDataCompressor{},
	Gzip: &GzipCompressor{},
}

// MaxMessageLength is the max length of a message.
// Default is 0 that means does not limit length of messages.
// It is used to validate when read messages from io.Reader.
var MaxMessageLength = 0

const (
	magicNumber byte = 0x08
)

func MagicNumber() byte {
	return magicNumber
}

var (
	// ErrMetaKVMissing some keys or values are missing.
	ErrMetaKVMissing = errors.New("wrong metadata lines. some keys or values are missing")
	// ErrMessageTooLong message is too long
	ErrMessageTooLong = errors.New("message is too long")

	ErrUnsupportedCompressor = errors.New("unsupported compressor")
)

const (
	// ServiceError contains error info of service invocation
	ServiceError = "__rpcx_error__"
)

// MessageType is message type of requests and responses.
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
	// Thrift
	// Thrift for payload
	Thrift
)

// Message is the generic type of Request and Response.
type Message struct {
	*Header
	ServicePath   string
	ServiceMethod string
	Metadata      map[string]string
	Payload       []byte
	data          []byte
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
	return MessageType(h[2]&0x80) >> 7
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
		h[2] = h[2] | 0x20
	} else {
		h[2] = h[2] &^ 0x20
	}
}

// CompressType returns compression type of messages.
func (h Header) CompressType() CompressType {
	return CompressType((h[2] & 0x1C) >> 2)
}

// SetCompressType sets the compression type.
func (h *Header) SetCompressType(ct CompressType) {
	h[2] = (h[2] &^ 0x1C) | ((byte(ct) << 2) & 0x1C)
}

// MessageStatusType returns the message status type.
func (h Header) MessageStatusType() MessageStatusType {
	return MessageStatusType(h[2] & 0x03)
}

// SetMessageStatusType sets message status type.
func (h *Header) SetMessageStatusType(mt MessageStatusType) {
	h[2] = (h[2] &^ 0x03) | (byte(mt) & 0x03)
}

// SerializeType returns serialization type of payload.
func (h Header) SerializeType() SerializeType {
	return SerializeType((h[3] & 0xF0) >> 4)
}

// SetSerializeType sets the serialization type.
func (h *Header) SetSerializeType(st SerializeType) {
	h[3] = (h[3] &^ 0xF0) | (byte(st) << 4)
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
	c := GetPooledMsg()
	header.SetCompressType(None)
	c.Header = &header
	c.ServicePath = m.ServicePath
	c.ServiceMethod = m.ServiceMethod
	return c
}

// Encode encodes messages.
func (m Message) Encode() []byte {
	data := m.EncodeSlicePointer()
	return *data
}

// EncodeSlicePointer encodes messages as a byte slice pointer we can use pool to improve.
func (m Message) EncodeSlicePointer() *[]byte {
	bb := bytebufferpool.Get()
	encodeMetadata(m.Metadata, bb)
	meta := bb.Bytes()

	spL := len(m.ServicePath)
	smL := len(m.ServiceMethod)

	var err error
	payload := m.Payload
	if m.CompressType() != None {
		compressor := Compressors[m.CompressType()]
		if compressor == nil {
			m.SetCompressType(None)
		} else {
			payload, err = compressor.Zip(m.Payload)
			if err != nil {
				m.SetCompressType(None)
				payload = m.Payload
			}
		}
	}

	totalL := (4 + spL) + (4 + smL) + (4 + len(meta)) + (4 + len(payload))

	// header + dataLen + spLen + sp + smLen + sm + metaL + meta + payloadLen + payload
	metaStart := 12 + 4 + (4 + spL) + (4 + smL)

	payLoadStart := metaStart + (4 + len(meta))
	l := 12 + 4 + totalL

	data := bufferPool.Get(l)
	copy(*data, m.Header[:])

	// totalLen
	binary.BigEndian.PutUint32((*data)[12:16], uint32(totalL))

	binary.BigEndian.PutUint32((*data)[16:20], uint32(spL))
	copy((*data)[20:20+spL], util.StringToSliceByte(m.ServicePath))

	binary.BigEndian.PutUint32((*data)[20+spL:24+spL], uint32(smL))
	copy((*data)[24+spL:metaStart], util.StringToSliceByte(m.ServiceMethod))

	binary.BigEndian.PutUint32((*data)[metaStart:metaStart+4], uint32(len(meta)))
	copy((*data)[metaStart+4:], meta)

	bytebufferpool.Put(bb)

	binary.BigEndian.PutUint32((*data)[payLoadStart:payLoadStart+4], uint32(len(payload)))
	copy((*data)[payLoadStart+4:], payload)

	return data
}

// PutData puts the byte slice into pool.
func PutData(data *[]byte) {
	bufferPool.Put(data)
}

// WriteTo writes message to writers.
func (m Message) WriteTo(w io.Writer) (int64, error) {
	nn, err := w.Write(m.Header[:])
	n := int64(nn)
	if err != nil {
		return n, err
	}

	bb := bytebufferpool.Get()
	encodeMetadata(m.Metadata, bb)
	meta := bb.Bytes()

	spL := len(m.ServicePath)
	smL := len(m.ServiceMethod)

	payload := m.Payload
	if m.CompressType() != None {
		compressor := Compressors[m.CompressType()]
		if compressor == nil {
			return n, ErrUnsupportedCompressor
		}
		payload, err = compressor.Zip(m.Payload)
		if err != nil {
			return n, err
		}
	}

	totalL := (4 + spL) + (4 + smL) + (4 + len(meta)) + (4 + len(payload))
	err = binary.Write(w, binary.BigEndian, uint32(totalL))
	if err != nil {
		return n, err
	}

	// write servicePath and serviceMethod
	err = binary.Write(w, binary.BigEndian, uint32(len(m.ServicePath)))
	if err != nil {
		return n, err
	}
	_, err = w.Write(util.StringToSliceByte(m.ServicePath))
	if err != nil {
		return n, err
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(m.ServiceMethod)))
	if err != nil {
		return n, err
	}
	_, err = w.Write(util.StringToSliceByte(m.ServiceMethod))
	if err != nil {
		return n, err
	}

	// write meta
	err = binary.Write(w, binary.BigEndian, uint32(len(meta)))
	if err != nil {
		return n, err
	}
	_, err = w.Write(meta)
	if err != nil {
		return n, err
	}

	bytebufferpool.Put(bb)

	// write payload
	err = binary.Write(w, binary.BigEndian, uint32(len(payload)))
	if err != nil {
		return n, err
	}

	nn, err = w.Write(payload)
	return int64(nn), err
}

// len,string,len,string,......
func encodeMetadata(m map[string]string, bb *bytebufferpool.ByteBuffer) {
	if len(m) == 0 {
		return
	}
	d := poolUint32Data.Get().(*[]byte)
	for k, v := range m {
		binary.BigEndian.PutUint32(*d, uint32(len(k)))
		bb.Write(*d)
		bb.Write(util.StringToSliceByte(k))
		binary.BigEndian.PutUint32(*d, uint32(len(v)))
		bb.Write(*d)
		bb.Write(util.StringToSliceByte(v))
	}
}

func decodeMetadata(l uint32, data []byte) (map[string]string, error) {
	m := make(map[string]string, 10)
	n := uint32(0)
	for n < l {
		// parse one key and value
		// key
		sl := binary.BigEndian.Uint32(data[n : n+4])
		n = n + 4
		if n+sl > l-4 {
			return m, ErrMetaKVMissing
		}
		k := string(data[n : n+sl])
		n = n + sl

		// value
		sl = binary.BigEndian.Uint32(data[n : n+4])
		n = n + 4
		if n+sl > l {
			return m, ErrMetaKVMissing
		}
		v := string(data[n : n+sl])
		n = n + sl
		m[k] = v
	}

	return m, nil
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
	// validate rest length for each step?

	// parse header
	_, err := io.ReadFull(r, m.Header[:1])
	if err != nil {
		return err
	}
	if !m.Header.CheckMagicNumber() {
		return fmt.Errorf("wrong magic number: %v", m.Header[0])
	}

	_, err = io.ReadFull(r, m.Header[1:])
	if err != nil {
		return err
	}

	// total
	lenData := poolUint32Data.Get().(*[]byte)
	_, err = io.ReadFull(r, *lenData)
	if err != nil {
		poolUint32Data.Put(lenData)
		return err
	}
	l := binary.BigEndian.Uint32(*lenData)
	poolUint32Data.Put(lenData)

	if MaxMessageLength > 0 && int(l) > MaxMessageLength {
		return ErrMessageTooLong
	}

	totalL := int(l)
	if cap(m.data) >= totalL { // reuse data
		m.data = m.data[:totalL]
	} else {
		m.data = make([]byte, totalL)
	}
	data := m.data
	_, err = io.ReadFull(r, data)
	if err != nil {
		return err
	}

	n := 0
	// parse servicePath
	l = binary.BigEndian.Uint32(data[n:4])
	n = n + 4
	nEnd := n + int(l)
	m.ServicePath = util.SliceByteToString(data[n:nEnd])
	n = nEnd

	// parse serviceMethod
	l = binary.BigEndian.Uint32(data[n : n+4])
	n = n + 4
	nEnd = n + int(l)
	m.ServiceMethod = util.SliceByteToString(data[n:nEnd])
	n = nEnd

	// parse meta
	l = binary.BigEndian.Uint32(data[n : n+4])
	n = n + 4
	nEnd = n + int(l)

	if l > 0 {
		m.Metadata, err = decodeMetadata(l, data[n:nEnd])
		if err != nil {
			return err
		}
	}
	n = nEnd

	// parse payload
	l = binary.BigEndian.Uint32(data[n : n+4])
	_ = l
	n = n + 4
	m.Payload = data[n:]

	if m.CompressType() != None {
		compressor := Compressors[m.CompressType()]
		if compressor == nil {
			return ErrUnsupportedCompressor
		}
		m.Payload, err = compressor.Unzip(m.Payload)
		if err != nil {
			return err
		}
	}

	return err
}

// Reset clean data of this message but keep allocated data
func (m *Message) Reset() {
	resetHeader(m.Header)
	m.Metadata = nil
	m.Payload = []byte{}
	m.data = m.data[:0]
	m.ServicePath = ""
	m.ServiceMethod = ""
}

var (
	zeroHeaderArray Header
	zeroHeader      = zeroHeaderArray[1:]
)

func resetHeader(h *Header) {
	copy(h[1:], zeroHeader)
}
