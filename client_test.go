package rpcx

import (
	"testing"
	"time"
)

func TestDirectSelector_Call(t *testing.T) {
	once.Do(startServer)

	s := &DirectClientSelector{Network: "tcp", Address: serverAddr, Timeout: 10 * time.Second}
	client := NewClient(s)

	args := &Args{7, 8}
	var reply Reply
	err := client.Call(serviceMethodName, args, &reply)
	if err != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	}

	client.Close()
}

func TestDirectSelector_Go(t *testing.T) {
	once.Do(startServer)

	s := &DirectClientSelector{Network: "tcp", Address: serverAddr, Timeout: 10 * time.Second}
	client := NewClient(s)

	args := &Args{7, 8}
	var reply Reply
	divCall := client.Go(serviceMethodName, args, &reply, nil)
	replyCall := <-divCall.Done // will be equal to divCall
	if replyCall.Error != nil {
		t.Errorf("error for Arith: %d*%d, %v \n", args.A, args.B, replyCall.Error)
	} else {
		t.Logf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

	client.Close()
}
