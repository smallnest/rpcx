package util

import (
	"maps"
	"unsafe"
)

func SliceByteToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func StringToSliceByte(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

func CopyMeta(src, dst map[string]string) {
	if dst == nil {
		return
	}
	maps.Copy(dst, src)
}
