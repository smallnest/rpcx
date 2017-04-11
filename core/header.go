package core

import (
	"net/textproto"
	"net/url"
)

// encodeHeader encodes a header as a string.
func encodeHeader(h Header) string {
	headerMap := (url.Values)(h)
	return headerMap.Encode()
}

// decodeHeader decodes a string as a Header.
func decodeHeader(s string) (Header, error) {
	v, err := url.ParseQuery(s)
	return Header(v), err
}

// A Header represents the key-value pairs in an RPCX header.
// It is imlemented refer to http Header.
type Header map[string][]string

// NewHeader returns a new header.
func NewHeader() Header {
	return make(map[string][]string)
}

// Add adds the key, value pair to the header.
// It appends to any existing values associated with key.
func (h Header) Add(key, value string) {
	textproto.MIMEHeader(h).Add(key, value)
}

// Len returns length of h.
func (h Header) Len() int {
	return len(h)
}

// Set sets the header entries associated with key to
// the single element value. It replaces any existing
// values associated with key.
func (h Header) Set(key, value string) {
	textproto.MIMEHeader(h).Set(key, value)
}

// Get gets the first value associated with the given key.
// It is case insensitive; textproto.CanonicalMIMEHeaderKey is used
// to canonicalize the provided key.
// If there are no values associated with the key, Get returns "".
// To access multiple values of a key, or to use non-canonical keys,
// access the map directly.
func (h Header) Get(key string) string {
	return textproto.MIMEHeader(h).Get(key)
}

// get is like Get, but key must already be in CanonicalHeaderKey form.
func (h Header) get(key string) string {
	if v := h[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Del deletes the values associated with key.
func (h Header) Del(key string) {
	textproto.MIMEHeader(h).Del(key)
}

func (h Header) clone() Header {
	h2 := make(Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

func (h Header) String() string {
	return encodeHeader(h)
}
