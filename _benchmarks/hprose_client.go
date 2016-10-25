package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hprose/hprose-golang/rpc"
	"github.com/montanaflynn/stats"
)

var concurrency = flag.Int("c", 1, "concurrency")
var total = flag.Int("n", 1, "total requests for all clients")
var host = flag.String("s", "127.0.0.1:8972", "server ip and port")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

type RO struct {
	Say func(args []byte) (reply []byte, err error) `simple:"true"`
}

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	n := *concurrency
	m := *total / n

	fmt.Printf("concurrency: %d\nrequests per client: %d\n\n", n, m)

	args := prepareArgs()

	b, _ := args.Marshal()
	fmt.Printf("message size: %d bytes\n\n", len(b))

	var wg sync.WaitGroup
	wg.Add(n * m)

	var startWg sync.WaitGroup
	startWg.Add(n)

	var trans uint64
	var transOK uint64

	d := make([][]int64, n, n)
	var ro *RO
	client := rpc.NewTCPClient("tcp://" + *host)
	client.SetMaxPoolSize(n)
	client.UseService(&ro)

	//it contains warmup time but we can ignore it
	totalT := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		dt := make([]int64, 0, m)
		d = append(d, dt)

		go func(i int) {
			reply := &BenchmarkMessage{}

			//warmup
			ro.Say(b)

			startWg.Done()
			startWg.Wait()

			for j := 0; j < m; j++ {
				t := time.Now().UnixNano()
				a, _ := args.Marshal()
				r, err := ro.Say(a)
				reply.Unmarshal(r)
				t = time.Now().UnixNano() - t

				d[i] = append(d[i], t)

				if err == nil && reply.Field1 == "OK" {
					atomic.AddUint64(&transOK, 1)
				}

				if err != nil {
					fmt.Println(err.Error())
				}

				atomic.AddUint64(&trans, 1)
				wg.Done()
			}

		}(i)

	}

	wg.Wait()
	client.Close()
	totalT = time.Now().UnixNano() - totalT
	totalT = totalT / 1000000
	fmt.Printf("took %d ms for %d requests", totalT, n*m)

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
	p99, _ := stats.Percentile(totalD2, 99.9)

	fmt.Printf("sent     requests    : %d\n", n*m)
	fmt.Printf("received requests    : %d\n", atomic.LoadUint64(&trans))
	fmt.Printf("received requests_OK : %d\n", atomic.LoadUint64(&transOK))
	fmt.Printf("throughput  (TPS)    : %d\n", int64(n*m)*1000/totalT)
	fmt.Printf("mean: %.f ns, median: %.f ns, max: %.f ns, min: %.f ns, p99.9: %.f ns\n", mean, median, max, min, p99)
	fmt.Printf("mean: %d ms, median: %d ms, max: %d ms, min: %d ms, p99: %d ms\n", int64(mean/1000000), int64(median/1000000), int64(max/1000000), int64(min/1000000), int64(p99/1000000))

}

func prepareArgs() *BenchmarkMessage {
	b := true
	var i int32 = 100000
	var s = "许多往事在眼前一幕一幕，变的那麼模糊"

	var args BenchmarkMessage

	v := reflect.ValueOf(&args).Elem()
	num := v.NumField()
	for k := 0; k < num; k++ {
		field := v.Field(k)
		if field.Type().Kind() == reflect.Ptr {
			switch v.Field(k).Type().Elem().Kind() {
			case reflect.Int, reflect.Int32, reflect.Int64:
				field.Set(reflect.ValueOf(&i))
			case reflect.Bool:
				field.Set(reflect.ValueOf(&b))
			case reflect.String:
				field.Set(reflect.ValueOf(&s))
			}
		} else {
			switch field.Kind() {
			case reflect.Int, reflect.Int32, reflect.Int64:
				field.SetInt(100000)
			case reflect.Bool:
				field.SetBool(true)
			case reflect.String:
				field.SetString(s)
			}
		}

	}
	return &args
}
