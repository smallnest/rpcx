package server

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Elem struct {
	// magicNumber the Magic Number
	magicNumber int
}

func (e Elem) Reset() {

}

func TestPool(t *testing.T) {
	elem := Elem{42}
	elemType := reflect.TypeOf(elem)
	// init Elem pool
	reflectTypePools.Init(elemType)
	reflectTypePools.Put(elemType, elem)
	// Get() will remove element from pool
	assert.Equal(t, elem.magicNumber, reflectTypePools.Get(elemType).(Elem).magicNumber)
}
