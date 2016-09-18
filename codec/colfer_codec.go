package codec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/rpc"
)

type colferMessage interface {
	MarshalTo(buf []byte) int
	MarshalLen() (int, error)
	MarshalBinary() (data []byte, err error)
	Unmarshal(data []byte) (int, error)
	UnmarshalBinary(data []byte) error
}

type colferClientCodec struct {
	conn    io.ReadWriteCloser
	Encoder *colferEncoder
	Decoder *colferDecoder
}

// NewColferClientCodec creates a bson ClientCodec
func NewColferClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return &colferClientCodec{
		conn:    conn,
		Encoder: newColferEncoder(conn),
		Decoder: newColferDecoder(conn),
	}
}

func (cc *colferClientCodec) WriteRequest(req *rpc.Request, v interface{}) (err error) {
	//convert request header
	rh := &RequestHeader{}
	rh.Method = req.ServiceMethod
	rh.Seq = req.Seq

	if err = cc.Encoder.Encode(rh); err != nil {
		return err
	}

	err = cc.Encoder.Encode(v)
	if err != nil {
		return
	}

	return
}

func (cc *colferClientCodec) ReadResponseHeader(res *rpc.Response) (err error) {
	//convert response header
	rh := &ResponseHeader{}

	if err = cc.Decoder.Decode(rh); err != nil {
		return err
	}

	res.ServiceMethod = rh.Method
	res.Seq = rh.Seq
	res.Error = rh.Error

	return
}

func (cc *colferClientCodec) ReadResponseBody(v interface{}) (err error) {
	err = cc.Decoder.Decode(v)
	return
}

func (cc *colferClientCodec) Close() (err error) {
	err = cc.conn.Close()
	return
}

type colferServerCodec struct {
	conn    io.ReadWriteCloser
	Encoder *colferEncoder
	Decoder *colferDecoder
}

// NewColferServerCodec creates a bson ServerCodec
func NewColferServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return &colferServerCodec{
		conn:    conn,
		Encoder: newColferEncoder(conn),
		Decoder: newColferDecoder(conn),
	}

}

func (sc *colferServerCodec) ReadRequestHeader(rq *rpc.Request) (err error) {
	//convert  colfer request header
	rh := &RequestHeader{}

	if err = sc.Decoder.Decode(rh); err != nil {
		return err
	}

	rq.ServiceMethod = rh.Method
	rq.Seq = rh.Seq

	return
}

func (sc *colferServerCodec) ReadRequestBody(v interface{}) (err error) {
	err = sc.Decoder.Decode(v)
	return
}

func (sc *colferServerCodec) WriteResponse(rs *rpc.Response, v interface{}) (err error) {
	//convert  colfer response header
	rp := &ResponseHeader{}
	rp.Method = rs.ServiceMethod
	rp.Seq = rs.Seq
	rp.Error = rs.Error

	err = sc.Encoder.Encode(rp)
	if err != nil {
		return
	}
	err = sc.Encoder.Encode(v)
	if err != nil {
		return
	}
	return
}

func (sc *colferServerCodec) Close() (err error) {
	err = sc.conn.Close()
	return
}

type colferEncoder struct {
	w io.Writer
}

func newColferEncoder(w io.Writer) *colferEncoder {
	return &colferEncoder{w: w}
}

func (e *colferEncoder) Encode(v interface{}) (err error) {
	msg := v.(colferMessage)

	//write size
	size, err := msg.MarshalLen()
	if err != nil {
		return
	}

	lenBuf := new(bytes.Buffer)
	binary.Write(lenBuf, binary.BigEndian, uint32(size))
	if _, err = e.w.Write(lenBuf.Bytes()); err != nil {
		return err
	}

	//write data
	buf := make([]byte, size)
	n := msg.MarshalTo(buf)
	if err != nil {
		return
	}

	n, err = e.w.Write(buf)
	if err != nil {
		return
	}

	if l := len(buf); n != l {
		err = fmt.Errorf("Wrote %d bytes, should have wrote %d", n, l)
	}

	return
}

type colferDecoder struct {
	r io.Reader
}

func newColferDecoder(r io.Reader) *colferDecoder {
	return &colferDecoder{r: r}
}

func (d *colferDecoder) Decode(pv interface{}) (err error) {
	var lbuf = make([]byte, 4)
	n, err := d.r.Read(lbuf)

	if n != 4 {
		err = fmt.Errorf("Corrupted colfer stream: could only read %d", n)
		return
	}
	if err != nil {
		return
	}

	size := (int(lbuf[3]) << 0) |
		(int(lbuf[2]) << 8) |
		(int(lbuf[1]) << 16) |
		(int(lbuf[0]) << 24)

	buf := make([]byte, size)
	n, err = d.r.Read(buf)
	//n, err := io.ReadFull(d.r, buf)
	if err != nil {
		return err
	}

	if n != size {
		return errors.New("wrong data size")
	}

	msg := pv.(colferMessage)
	err = msg.UnmarshalBinary(buf)
	return
}
