package server

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPool(t *testing.T) {
	// not pool anything yet
	UsePool = false
	// THE Magic Number
	magicNumber := 42

	intType := reflect.TypeOf(magicNumber)
	// init int pool
	reflectTypePools.Init(intType)
	// insert a integer
	reflectTypePools.Put(intType, magicNumber)
	// if UsePool == false, argsReplyPools.Get() will call reflect.New() which
	// returns a Value representing a pointer to a new zero value
	assert.Equal(t, 0, *reflectTypePools.Get(intType).(*int))

	// start pooling
	UsePool = true

	reflectTypePools.Put(intType, magicNumber)
	// Get() will remove element from pool
	assert.Equal(t, magicNumber, reflectTypePools.Get(intType).(int))
}
