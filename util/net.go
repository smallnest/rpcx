package util

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// GetFreePort gets a free port.
func GetFreePort() (port int, err error) {
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

// ParseRpcxAddress parses rpcx address such as tcp@127.0.0.1:8972  quic@192.168.1.1:9981
func ParseRpcxAddress(addr string) (network string, ip string, port int, err error) {
	ati := strings.Index(addr, "@")
	if ati <= 0 {
		return "", "", 0, fmt.Errorf("invalid rpcx address: %s", addr)
	}

	network = addr[:ati]
	addr = addr[ati+1:]

	var portstr string
	ip, portstr, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", 0, err
	}

	port, err = strconv.Atoi(portstr)
	return network, ip, port, err
}

func ConvertMeta2Map(meta string) map[string]string {
	var rt = make(map[string]string)

	if meta == "" {
		return rt
	}

	v, err := url.ParseQuery(meta)
	if err != nil {
		return rt
	}

	for key := range v {
		rt[key] = v.Get(key)
	}
	return rt
}

func ConvertMap2String(meta map[string]string) string {
	var buf strings.Builder
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vs := meta[k]
		keyEscaped := url.QueryEscape(k)
		if buf.Len() > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(keyEscaped)
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(vs))
	}
	return buf.String()
}
