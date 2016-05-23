package rpcx

import (
	"io"
	"time"
	"unsafe"
)

var (
	_ = unsafe.Sizeof(0)
	_ = io.ReadFull
	_ = time.Now()
)

type GencodeArgs struct {
	A int32
	B int32
}

func (d *GencodeArgs) Size() (s uint64) {

	s += 8
	return
}
func (d *GencodeArgs) Marshal(buf []byte) ([]byte, error) {
	size := d.Size()
	{
		if uint64(cap(buf)) >= size {
			buf = buf[:size]
		} else {
			buf = make([]byte, size)
		}
	}
	i := uint64(0)

	{

		buf[0+0] = byte(d.A >> 0)

		buf[1+0] = byte(d.A >> 8)

		buf[2+0] = byte(d.A >> 16)

		buf[3+0] = byte(d.A >> 24)

	}
	{

		buf[0+4] = byte(d.B >> 0)

		buf[1+4] = byte(d.B >> 8)

		buf[2+4] = byte(d.B >> 16)

		buf[3+4] = byte(d.B >> 24)

	}
	return buf[:i+8], nil
}

func (d *GencodeArgs) Unmarshal(buf []byte) (uint64, error) {
	i := uint64(0)

	{

		d.A = 0 | (int32(buf[0+0]) << 0) | (int32(buf[1+0]) << 8) | (int32(buf[2+0]) << 16) | (int32(buf[3+0]) << 24)

	}
	{

		d.B = 0 | (int32(buf[0+4]) << 0) | (int32(buf[1+4]) << 8) | (int32(buf[2+4]) << 16) | (int32(buf[3+4]) << 24)

	}
	return i + 8, nil
}

type GencodeReply struct {
	C int32
}

func (d *GencodeReply) Size() (s uint64) {

	s += 4
	return
}
func (d *GencodeReply) Marshal(buf []byte) ([]byte, error) {
	size := d.Size()
	{
		if uint64(cap(buf)) >= size {
			buf = buf[:size]
		} else {
			buf = make([]byte, size)
		}
	}
	i := uint64(0)

	{

		buf[0+0] = byte(d.C >> 0)

		buf[1+0] = byte(d.C >> 8)

		buf[2+0] = byte(d.C >> 16)

		buf[3+0] = byte(d.C >> 24)

	}
	return buf[:i+4], nil
}

func (d *GencodeReply) Unmarshal(buf []byte) (uint64, error) {
	i := uint64(0)

	{

		d.C = 0 | (int32(buf[0+0]) << 0) | (int32(buf[1+0]) << 8) | (int32(buf[2+0]) << 16) | (int32(buf[3+0]) << 24)

	}
	return i + 4, nil
}
