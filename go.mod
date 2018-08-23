module github.com/smallnest/rpcx

require (
	git.apache.org/thrift.git v0.0.0-20180807212849-6e67faa92827
	github.com/RoaringBitmap/roaring v0.4.16 // indirect
	github.com/anacrolix/sync v0.0.0-20180808010631-44578de4e778 // indirect
	github.com/anacrolix/tagflag v0.0.0-20180803105420-3a8ff5428f76 // indirect
	github.com/anacrolix/utp v0.0.0-20180219060659-9e0e1d1d0572
	github.com/bifurcation/mint v0.0.0-20180715133206-93c51c6ce115 // indirect
	github.com/cenk/backoff v2.0.0+incompatible // indirect
	github.com/cenkalti/backoff v2.0.0+incompatible // indirect
	github.com/cheekybits/genny v0.0.0-20180817214931-a9c2292b062a // indirect
	github.com/coreos/etcd v3.3.9+incompatible // indirect
	github.com/coreos/go-semver v0.2.0 // indirect
	github.com/docker/libkv v0.2.1
	github.com/dustin/go-humanize v0.0.0-20180713052910-9f541cc9db5d // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/fatih/color v1.7.0
	github.com/gogo/protobuf v1.1.1
	github.com/golang/protobuf v1.2.0
	github.com/google/btree v0.0.0-20180813153112-4030bb1f1f0c // indirect
	github.com/grandcat/zeroconf v0.0.0-20180329153754-df75bb3ccae1
	github.com/hashicorp/consul v1.2.2 // indirect
	github.com/hashicorp/errwrap v0.0.0-20180715044906-d6c0cd880357 // indirect
	github.com/hashicorp/go-cleanhttp v0.0.0-20171218145408-d5fe4b57a186 // indirect
	github.com/hashicorp/go-multierror v0.0.0-20180717150148-3d5d8f294aa0
	github.com/hashicorp/go-rootcerts v0.0.0-20160503143440-6bb64b370b90 // indirect
	github.com/hashicorp/golang-lru v0.0.0-20180201235237-0fb14efe8c47 // indirect
	github.com/hashicorp/serf v0.8.1 // indirect
	github.com/influxdata/influxdb v1.6.1 // indirect
	github.com/juju/ratelimit v1.0.1
	github.com/julienschmidt/httprouter v0.0.0-20180715161854-348b672cd90d
	github.com/kavu/go_reuseport v1.3.0
	github.com/klauspost/cpuid v0.0.0-20180405133222-e7e905edc00e // indirect
	github.com/klauspost/crc32 v0.0.0-20170628072449-bab58d77464a // indirect
	github.com/klauspost/reedsolomon v0.0.0-20180704173009-925cb01d6510 // indirect
	github.com/lucas-clemente/aes12 v0.0.0-20171027163421-cd47fb39b79f // indirect
	github.com/lucas-clemente/quic-go v0.9.0 // indirect
	github.com/lucas-clemente/quic-go-certificates v0.0.0-20160823095156-d2f86524cced // indirect
	github.com/marten-seemann/quic-conn v0.0.0-20180707100625-158b6bb88fb9
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.3 // indirect
	github.com/miekg/dns v1.0.8 // indirect
	github.com/mitchellh/mapstructure v0.0.0-20180715050151-f15292f7a699 // indirect
	github.com/pkg/errors v0.8.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20180503174638-e2704e165165
	github.com/rubyist/circuitbreaker v2.2.1+incompatible
	github.com/samuel/go-zookeeper v0.0.0-20180130194729-c4fab1ac1bec // indirect
	github.com/soheilhy/cmux v0.1.4
	github.com/stretchr/testify v1.2.2 // indirect
	github.com/tatsushid/go-fastping v0.0.0-20160109021039-d7bb493dee3e
	github.com/u35s/rudp v0.0.0-20171228014240-b384c469e861
	github.com/ugorji/go v1.1.1 // indirect
	github.com/valyala/fastrand v0.0.0-20170531153657-19dd0f0bf014
	github.com/vmihailenco/msgpack v3.3.3+incompatible
	github.com/vrischmann/go-metrics-influxdb v0.0.0-20160917065939-43af8332c303
	github.com/xtaci/kcp-go v2.0.3+incompatible
	golang.org/x/crypto v0.0.0-20180820150726-614d502a4dac // indirect
	golang.org/x/net v0.0.0-20180821023952-922f4815f713
	golang.org/x/text v0.3.0 // indirect
)

replace (
	golang.org/x/crypto v0.0.0-20180820150726-614d502a4dac => github.com/golang/crypto v0.0.0-20180820150726-614d502a4dac
	golang.org/x/net v0.0.0-20180821023952-922f4815f713 => github.com/golang/net v0.0.0-20180821023952-922f4815f713
	golang.org/x/text v0.3.0 => github.com/golang/text v0.3.0
)
