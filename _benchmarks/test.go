package main

import (
	"fmt"
	"reflect"
)

type S struct {
	I0   int
	I1   *int
	Str0 string
	Str1 *string
}

func main() {
	s := new(S)

	v := reflect.ValueOf(s).Elem()

	fmt.Printf("%+v\n", v.Field(0).CanSet())
	fmt.Printf("%+v\n", v.Field(1).CanSet())

	//0: I0
	v.Field(0).SetInt(100)

	//1: I1
	i := 200
	v.Field(1).Set(reflect.ValueOf(&i))

	//1: I1
	intPtrField := reflect.Indirect(v.Field(1))
	intPtrField.SetInt(300)

	//2: Str0
	v.Field(2).SetString("hello")

	//3: Str1
	str := "world"
	v.Field(3).Set(reflect.ValueOf(&str))

	//3: Str1
	strPtrField := reflect.Indirect(v.Field(3))
	strPtrField.SetString("世界")

	fmt.Printf("%+v\n", *s.Str1)
}
