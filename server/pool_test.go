package server

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPool(t *testing.T) {
	// THE Magic Number
	magicNumber := 42

	intType := reflect.TypeOf(magicNumber)
	// init int pool
	reflectTypePools.Init(intType)

	reflectTypePools.Put(intType, magicNumber)
	// Get() will remove element from pool
	assert.Equal(t, magicNumber, reflectTypePools.Get(intType).(int))
}
