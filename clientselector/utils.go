package clientselector

import (
	"fmt"
	"hash/fnv"
)

// Hash consistently chooses a hash bucket number in the range [0, numBuckets) for the given key. numBuckets must be >= 1.
func Hash(key uint64, buckets int32) int32 {
	if buckets <= 0 {
		buckets = 1
	}

	var b, j int64

	for j < int64(buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(int64(1)<<31) / float64((key>>33)+1)))
	}

	return int32(b)
}

// HashString get a hash value of a string
func HashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// HashServiceAndArgs define a hash function
type HashServiceAndArgs func(len int, options ...interface{}) int

// JumpConsistentHash selects a server by serviceMethod and args
func JumpConsistentHash(len int, options ...interface{}) int {
	keyString := ""
	for _, opt := range options {
		keyString = keyString + "/" + toString(opt)
	}
	key := HashString(keyString)
	return int(Hash(key, int32(len)))
}

func toString(obj interface{}) string {
	return fmt.Sprintf("%v", obj)
}
