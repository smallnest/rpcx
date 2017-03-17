package util

import (
	"reflect"
	"unsafe"
)

func SliceByteToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func StringToSliceByte(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	ss := (*reflect.StringHeader)(unsafe.Pointer(&s))
	return (*[0x7fffffff]byte)(unsafe.Pointer(ss.Data))[:len(s):len(s)]
}
