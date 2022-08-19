package util

import (
	"net"
	"strconv"
	"testing"
)

func TestGetFreePort(t *testing.T) {
	for i := 0; i < 1000; i++ {
		port, err := GetFreePort()
		if err != nil {
			t.Error(err)
		}

		if port == 0 {
			t.Error("GetFreePort() return 0")
		}
	}
}

var oldGetFreePort = func() (port int, err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().String()
	_, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(portString)
}

func BenchmarkGetFreePort_Old(b *testing.B) {
	for i := 0; i < b.N; i++ {
		oldGetFreePort()
	}
}

func BenchmarkGetFreePort_New(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetFreePort()
	}
}

func TestExternalIPV4(t *testing.T) {
	ip, err := ExternalIPV4()
	if err != nil {
		t.Fatal(err)
	}

	if ip == "" {
		t.Fatal("expect an IP but got empty")
	}
	t.Log(ip)
}

func TestExternalIPV6(t *testing.T) {
	ip, err := ExternalIPV6()
	if err != nil {
		t.Fatal(err)
	}

	if ip == "" {
		t.Fatal("expect an IP but got empty")
	}
	t.Log(ip)
}
