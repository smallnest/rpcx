package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/smallnest/rpcx"
	"github.com/smallnest/rpcx/codec"
)

var concurrency = flag.Int("c", 1, "concurrency")
var total = flag.Int("n", 1, "total requests for all clients")

func main() {
	flag.Parse()
	n := *concurrency
	m := *total / n

	fmt.Printf("concurrency: %d\n requests per client: %d\n\n", n, m)

	serviceMethodName := "Hello.Say"
	args := prepareArgs()

	var wg sync.WaitGroup
	wg.Add(n * m)

	var trans uint64
	var transOK uint64

	d := make([][]int64, n, n)

	for i := 0; i < n; i++ {
		dt := make([]int64, 0, m)
		d = append(d, dt)

		go func() {
			s := &rpcx.DirectClientSelector{Network: "tcp", Address: "127.0.0.1:8972"}
			client := rpcx.NewClient(s)
			client.ClientCodecFunc = codec.NewProtobufClientCodec

			var reply BenchmarkMessage

			//warmup
			for j := 0; j < 5; j++ {
				client.Call(serviceMethodName, args, &reply)
			}

			for j := 0; j < m; j++ {
				t := time.Now().UnixNano()
				err := client.Call(serviceMethodName, args, &reply)
				t = time.Now().UnixNano() - t

				d[i] = append(d[i], t)

				if err == nil && reply.Field1 == "OK" {
					atomic.AddUint64(&transOK, 1)
				}

				atomic.AddUint64(&trans, 1)
				wg.Done()
			}

			client.Close()

		}()

	}

	wg.Wait()

	totalD := make([]int64, 0, n*m)
	for _, k := range d {
		totalD = append(totalD, k...)
	}
	totalD2 := make([]float64, 0, n*m)
	for _, k := range totalD {
		totalD2 = append(totalD2, float64(k))
	}

	mean, _ := stats.Mean(totalD2)
	median, _ := stats.Median(totalD2)
	max, _ := stats.Max(totalD2)
	min, _ := stats.Min(totalD2)

	fmt.Printf("sent     requests    : %d\n", n*m)
	fmt.Printf("received requests    : %d\n", atomic.LoadUint64(&trans))
	fmt.Printf("received requests_OK : %d\n", atomic.LoadUint64(&transOK))
	fmt.Printf("mean: %.f ns, median: %.f ns, max: %.f ns, min: %.f ns\n", mean, median, max, min)
	fmt.Printf("mean: %d ms, median: %d ms, max: %d ms, min: %d ms\n", int64(mean/1000000), int64(median/1000000), int64(max/1000000), int64(min/1000000))

}

func prepareArgs() *BenchmarkMessage {
	b := true
	var i int32 = 100032
	var j int64 = 10000064
	var s = "许多往事在眼前一幕一幕，变的那麼模糊"

	var args BenchmarkMessage
	args.Field1 = s
	args.Field100 = &i
	args.Field101 = &i
	args.Field102 = s
	args.Field104 = &i
	args.Field12 = &b
	args.Field128 = &i
	args.Field129 = &s
	args.Field13 = &b
	args.Field130 = &i
	args.Field131 = &i
	args.Field14 = &b
	args.Field150 = i
	args.Field16 = i
	args.Field17 = &b
	args.Field18 = s

	args.Field2 = i
	args.Field22 = j
	args.Field23 = &i
	args.Field24 = &b
	args.Field25 = &i
	args.Field271 = &i
	args.Field272 = &i
	args.Field280 = i

	args.Field3 = i
	args.Field30 = &b
	args.Field4 = s
	args.Field6 = &i
	args.Field60 = &i
	args.Field67 = &i
	args.Field68 = i

	args.Field7 = s
	args.Field78 = b
	args.Field80 = &b
	args.Field81 = &b
	args.Field9 = s

	return &args
}
