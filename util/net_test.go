package util

import "testing"

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
