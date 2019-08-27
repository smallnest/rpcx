module github.com/smallnest/rpcx

require (
	github.com/abronan/valkeyrie v0.0.0-20190822142731-f2e1850dc905
	github.com/anacrolix/envpprof v1.0.1 // indirect
	github.com/anacrolix/sync v0.0.0-20180808010631-44578de4e778 // indirect
	github.com/anacrolix/utp v0.0.0-20180219060659-9e0e1d1d0572
	github.com/apache/thrift v0.12.0
	github.com/cenk/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/coreos/etcd v3.3.15+incompatible // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/docker/libkv v0.2.1
	github.com/edwingeng/doublejump v0.0.0-20190102103700-461a0155c7be
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/fatih/color v1.7.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/mock v1.3.1 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/grandcat/zeroconf v0.0.0-20190424104450-85eadb44205c
	github.com/grpc-ecosystem/grpc-gateway v1.9.6 // indirect
	github.com/hashicorp/consul/api v1.2.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/go-rootcerts v1.0.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.3
	github.com/hashicorp/serf v0.8.3 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/influxdata/influxdb1-client v0.0.0-20190809212627-fc22c7df067e
	github.com/juju/ratelimit v1.0.1
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kavu/go_reuseport v1.4.0
	github.com/klauspost/cpuid v1.2.1 // indirect
	github.com/klauspost/reedsolomon v1.9.2 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/lucas-clemente/quic-go v0.12.0 // indirect
	github.com/marten-seemann/quic-conn v0.0.0-20190528165812-50224736eb42
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/miekg/dns v1.1.16 // indirect
	github.com/onsi/ginkgo v1.9.0 // indirect
	github.com/onsi/gomega v1.6.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/peterbourgon/g2s v0.0.0-20170223122336-d4e7ad98afea // indirect
	github.com/prometheus/procfs v0.0.4 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20190826022208-cac0b30c2563
	github.com/rs/cors v1.7.0
	github.com/rubyist/circuitbreaker v2.2.1+incompatible
	github.com/samuel/go-zookeeper v0.0.0-20190810000440-0ceca61e4d75 // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/smallnest/libkv-etcdv3-store v0.0.0-20190827025031-6b33215321f0
	github.com/soheilhy/cmux v0.1.4
	github.com/tatsushid/go-fastping v0.0.0-20160109021039-d7bb493dee3e
	github.com/templexxx/cpufeat v0.0.0-20180724012125-cef66df7f161 // indirect
	github.com/templexxx/xor v0.0.0-20181023030647-4e92f724b73b // indirect
	github.com/tjfoc/gmsm v1.0.1 // indirect
	github.com/valyala/fastrand v1.0.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/xtaci/kcp-go v5.4.4+incompatible
	github.com/xtaci/lossyconn v0.0.0-20190602105132-8df528c0c9ae // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	go.etcd.io/etcd v3.3.15+incompatible // indirect
	go.opencensus.io v0.22.0
	golang.org/x/crypto v0.0.0-20190820162420-60c769a6c586 // indirect
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7 // indirect
	golang.org/x/sys v0.0.0-20190826190057-c7b8b68b1456 // indirect
	golang.org/x/time v0.0.0-20190513212739-9d24e82272b4 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	google.golang.org/genproto v0.0.0-20190819201941-24fa4b261c55 // indirect
	google.golang.org/grpc v1.23.0 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace (
	google.golang.org/appengine v1.1.0 => github.com/golang/appengine v1.1.0
	google.golang.org/genproto v0.0.0-20180831171423-11092d34479b => github.com/google/go-genproto v0.0.0-20180831171423-11092d34479b
	google.golang.org/grpc v1.14.0 => github.com/grpc/grpc-go v1.14.0
)
