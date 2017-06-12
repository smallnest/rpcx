package rpcx

import (
	"context"
	"testing"
	"time"

	cryrand "crypto/rand"
	"crypto/sha1"
	kcp "github.com/xtaci/kcp-go"
	"golang.org/x/crypto/pbkdf2"
)

var kcpServerAddr string
var kcpServer *Server

var pass = []byte("testpass")
var salt []byte = CryptoRandBytes(64)

// at 1 million iter, takes about 1.5 seconds to generate the key.
// That's a good production value. For test we do much less, only 1024.
//const iter = 1 << 20
const iter = 1 << 10
const keylen = 32

var key = pbkdf2.Key(pass, salt, iter, keylen, sha1.New)

func CryptoRandBytes(n int) []byte {
	b := make([]byte, n)
	_, err := cryrand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func startKcpServer() {
	blockCrypt, err := kcp.NewSalsa20BlockCrypt(key)
	panicOn(err)

	kcpServer = NewServer()
	kcpServer.KCPConfig.BlockCrypt = blockCrypt
	kcpServer.RegisterName(serviceName, service)
	kcpServer.Start("kcp", "127.0.0.1:0")
	kcpServerAddr = kcpServer.Address()
}

func startKcpClient(t *testing.T) {

	blockCrypt, err := kcp.NewSalsa20BlockCrypt(key)
	panicOn(err)
	s := &DirectClientSelector{Network: "kcp", Address: kcpServerAddr, DialTimeout: 10 * time.Second}
	client := NewClient(s)
	client.Block = blockCrypt
	defer client.Close()

	args := &Args{7, 8}
	var reply Reply

	divCall := client.Go(context.Background(), serviceMethodName, args, &reply, nil)
	replyCall := <-divCall.Done // will be equal to divCall
	if replyCall.Error != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, replyCall.Error)
	} else {
		t.Logf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}
}

func TestKcpServe(t *testing.T) {
	startKcpServer()

	// Give the server a moment to begin listening on that udp port.
	time.Sleep(10 * time.Millisecond)
	startKcpClient(t)
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}
