package server

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOption(t *testing.T) {
	server := NewServer()

	cert, _ := tls.LoadX509KeyPair("server.pem", "server.key")
	config := &tls.Config{Certificates: []tls.Certificate{cert}}

	o := WithTLSConfig(config)
	o(server)
	assert.Equal(t, config, server.tlsConfig)

	o = WithReadTimeout(time.Second)
	o(server)
	assert.Equal(t, time.Second, server.readTimeout)

	o = WithWriteTimeout(time.Second)
	o(server)
	assert.Equal(t, time.Second, server.writeTimeout)
}

