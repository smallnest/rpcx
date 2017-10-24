package protocol

import "sync"

// var maxBytes = 10240
// var poolRequestDada = sync.Pool{
// 	New: func() interface{} {
// 		return make([]byte, maxBytes)
// 	},
// }

// func getBytes(l int) []byte {
// 	b := poolRequestDada.Get().([]byte)
// 	if len(b) < l {
// 		poolRequestDada.Put(b)
// 		return make([]byte, l)
// 	}

// 	return b[:l]
// }

// func freeBytes(b []byte) {
// 	if len(b) <= maxBytes {
// 		poolRequestDada.Put(b)
// 	}
// }

var poolUint32Dada = sync.Pool{
	New: func() interface{} {
		data := make([]byte, 4)
		return &data
	},
}
