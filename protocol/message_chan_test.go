package protocol

import (
    "fmt"
    "strings"
    "sync/atomic"
    "testing"
    "time"
)

func TestChanValue(t *testing.T) {
    var ct uint64
    ch := make(chan Message, 10000)
    go func(ch <-chan Message) {
        for range ch {
            atomic.AddUint64(&ct, 1)
        }
    }(ch)

    go func(ch chan Message) {
        m := strings.Repeat("_", 100)
        p := strings.Repeat("_", 100)
        payload := make([]byte, 1024)
        for {
            ch <- Message{ServiceMethod: m, ServicePath: p, Payload: payload}
        }
    }(ch)
    for i := 0; i < 5; i++ {
        time.Sleep(time.Second)
        fmt.Println(atomic.LoadUint64(&ct))
        atomic.StoreUint64(&ct, 0)
    }
}

func TestChanPtr(t *testing.T) {
    var ct uint64
    ch := make(chan *Message, 10000)
    go func(ch <-chan *Message) {
        for range ch {
            atomic.AddUint64(&ct, 1)
        }
    }(ch)

    go func(ch chan *Message) {
        m := strings.Repeat("_", 100)
        p := strings.Repeat("_", 100)
        payload := make([]byte, 1024)
        for {
            ch <- &Message{ServiceMethod: m, ServicePath: p,Payload: payload}
        }
    }(ch)

    for i := 0; i < 5; i++ {
        time.Sleep(time.Second)
        fmt.Println(atomic.LoadUint64(&ct))
        atomic.StoreUint64(&ct, 0)
    }
}

func BenchmarkChanValue(b *testing.B) {
    ch := make(chan Message, 10000)
    var ct uint64
    go func(ch <-chan Message) {
        for range ch {
            atomic.AddUint64(&ct, 1)
        }
    }(ch)
    b.ReportAllocs()
    b.ResetTimer()
    m := strings.Repeat("_", 100)
    p := strings.Repeat("_", 100)
    payload := make([]byte, 1024)
    for i := 0; i < b.N; i++ {
        ch <- Message{ServiceMethod: m, ServicePath: p,Payload: payload}
    }
}

func BenchmarkChanPtr(b *testing.B) {
    ch := make(chan *Message, 10000)
    var ct uint64
    go func(ch <-chan *Message) {
        for range ch {
            atomic.AddUint64(&ct, 1)
        }
    }(ch)
    b.ReportAllocs()
    b.ResetTimer()
    m := strings.Repeat("_", 100)
    p := strings.Repeat("_", 100)
    payload := make([]byte, 1024)
    for i := 0; i < b.N; i++ {
        ch <- &Message{ServiceMethod: m, ServicePath: p,Payload: payload}
    }
}