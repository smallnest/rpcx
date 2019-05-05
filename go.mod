module github.com/smallnest/rpcx

require (
	github.com/anacrolix/envpprof v1.0.0 // indirect
	github.com/anacrolix/missinggo v1.1.0 // indirect
	github.com/anacrolix/sync v0.0.0-20180808010631-44578de4e778 // indirect
	github.com/anacrolix/utp v0.0.0-20180219060659-9e0e1d1d0572
	github.com/apache/thrift v0.12.0
	github.com/bradfitz/iter v0.0.0-20190303215204-33e6a9893b0c // indirect
	github.com/cenk/backoff v2.1.1+incompatible // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible // indirect
	github.com/docker/libkv v0.2.1
	github.com/edwingeng/doublejump v0.0.0-20190102103700-461a0155c7be
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/fatih/color v1.7.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.1
	github.com/grandcat/zeroconf v0.0.0-20190424104450-85eadb44205c
	github.com/hashicorp/consul/api v1.0.1 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.1
	github.com/influxdata/influxdb1-client v0.0.0-20190402204710-8ff2fc3824fc
	github.com/juju/ratelimit v1.0.1
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kavu/go_reuseport v1.4.0
	github.com/klauspost/cpuid v1.2.1 // indirect
	github.com/klauspost/reedsolomon v1.9.1 // indirect
	github.com/marten-seemann/quic-conn v0.0.0-20190404134349-539f7de6a079
	github.com/mattn/go-colorable v0.1.1 // indirect
	github.com/mattn/go-isatty v0.0.7 // indirect
	github.com/miekg/dns v1.1.9 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/peterbourgon/g2s v0.0.0-20170223122336-d4e7ad98afea // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/rs/cors v1.6.0
	github.com/rubyist/circuitbreaker v2.2.1+incompatible
	github.com/samuel/go-zookeeper v0.0.0-20180130194729-c4fab1ac1bec // indirect
	github.com/soheilhy/cmux v0.1.4
	github.com/tatsushid/go-fastping v0.0.0-20160109021039-d7bb493dee3e
	github.com/templexxx/cpufeat v0.0.0-20180724012125-cef66df7f161 // indirect
	github.com/templexxx/xor v0.0.0-20181023030647-4e92f724b73b // indirect
	github.com/tjfoc/gmsm v1.0.1 // indirect
	github.com/u35s/rudp v0.0.0-20171228014240-b384c469e861
	github.com/valyala/fastrand v1.0.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/xtaci/kcp-go v5.2.8+incompatible
	go.opencensus.io v0.21.0
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
)

replace (
	google.golang.org/appengine v1.1.0 => github.com/golang/appengine v1.1.0
	google.golang.org/genproto v0.0.0-20180831171423-11092d34479b => github.com/google/go-genproto v0.0.0-20180831171423-11092d34479b
	google.golang.org/grpc v1.14.0 => github.com/grpc/grpc-go v1.14.0
)
