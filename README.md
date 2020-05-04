<a href="https://rpcx.io/"><img height="160" src="http://rpcx.io/logos/rpcx-logo-text.png"></a>

Official site: [http://rpcx.io](http://rpcx.io/)

[![License](https://img.shields.io/:license-apache%202-blue.svg)](https://opensource.org/licenses/Apache-2.0) [![GoDoc](https://godoc.org/github.com/smallnest/rpcx?status.png)](http://godoc.org/github.com/smallnest/rpcx)  [![travis](https://travis-ci.org/smallnest/rpcx.svg?branch=master)](https://travis-ci.org/smallnest/rpcx) [![Go Report Card](https://goreportcard.com/badge/github.com/smallnest/rpcx)](https://goreportcard.com/report/github.com/smallnest/rpcx) [![coveralls](https://coveralls.io/repos/smallnest/rpcx/badge.svg?branch=master&service=github)](https://coveralls.io/github/smallnest/rpcx?branch=master) [![QQ2群](https://img.shields.io/:QQ2群-670248151-blue.svg)](_documents/rpcx_dev_qq2.png) [![QQ群(已满)](https://img.shields.io/:QQ群(已满)-398044387-blue.svg)](_documents/rpcx_dev_qq.png) 

## Announce

A tcpdump-like tool added: [rpcxdump](https://github.com/smallnest/rpcxdump)。 You can use it to debug communications between rpcx services and clients.

![](https://github.com/smallnest/rpcxdump/blob/master/snapshoot.png)


## Cross-Languages
you can use other programming languages besides Go to access rpcx services.

- **rpcx-gateway**: You can write clients in any programming languages to call rpcx services via [rpcx-gateway](https://github.com/rpcxio/rpcx-gateway)
- **http invoke**: you can use the same http requests to access rpcx gateway
- **Java Services/Clients**: You can use [rpcx-java](https://github.com/smallnest/rpcx-java) to implement/access rpcx servies via raw protocol.


> If you can write Go methods, you can also write rpc services. It is so easy to write rpc applications with rpcx.

## Installation

install the basic features:

`go get -v github.com/smallnest/rpcx/...`


If you want to use `ping`、`quic`、`kcp`、`utp` registry, use those tags to `go get` 、 `go build` or `go run`. For example, if you want to use all features, you can:

```sh
go get -v -tags "quic kcp ping utp" github.com/smallnest/rpcx/...
```

**_tags_**:
- **quic**: support quic transport
- **kcp**: support kcp transport
- **ping**: support network quality load balancing
- **utp**: support utp transport

## Features
rpcx is a RPC framework like [Alibaba Dubbo](http://dubbo.io/) and [Weibo Motan](https://github.com/weibocom/motan).

**rpcx 3.0** has been refactored for targets:
1. **Simple**: easy to learn, easy to develop, easy to intergate and easy to deploy
2. **Performance**: high perforamnce (>= grpc-go)
3. **Cross-platform**: support _raw slice of bytes_, _JSON_, _Protobuf_ and _MessagePack_. Theoretically it can be used with java, php, python, c/c++, node.js, c# and other platforms
4. **Service discovery and service governance**: support zookeeper, etcd and consul.


It contains below features
- Support raw Go functions. There's no need to define proto files.
- Pluggable. Features can be extended such as service discovery, tracing.
- Support TCP, HTTP, [QUIC](https://en.wikipedia.org/wiki/QUIC) and [KCP](https://github.com/skywind3000/kcp)
- Support multiple codecs such as JSON, Protobuf, [MessagePack](https://msgpack.org/index.html) and raw bytes.
- Service discovery. Support peer2peer, configured peers, [zookeeper](https://zookeeper.apache.org), [etcd](https://github.com/coreos/etcd), [consul](https://www.consul.io) and [mDNS](https://en.wikipedia.org/wiki/Multicast_DNS).
- Fault tolerance：Failover, Failfast, Failtry.
- Load banlancing：support Random, RoundRobin, Consistent hashing, Weighted, network quality and Geography.
- Support Compression.
- Support passing metadata.
- Support Authorization.
- Support heartbeat and one-way request.
- Other features: metrics, log, timeout, alias, circuit breaker.
- Support bidirectional communication.
- Support access via HTTP so you can write clients in any programming languages.
- Support API gateway.
- Support backup request, forking and broadcast.


rpcx uses a binary protocol and platform-independent, which means you can develop services in other languages such as Java, python, nodejs, and you can use other prorgramming languages to invoke services developed in Go.

There is a UI manager: [rpcx-ui](https://github.com/smallnest/rpcx-ui).

## Performance

Test results show rpcx has better performance than other rpc framework except standard rpc lib.


The benchmark code is at [rpcx-benchmark](https://github.com/rpcx-ecosystem/rpcx-benchmark).

**Listen to others, but test by yourself**.

**_Test Environment_**

- **CPU**: Intel(R) Xeon(R) CPU E5-2630 v3 @ 2.40GHz, 32 cores
- **Memory**: 32G
- **Go**: 1.9.0
- **OS**: CentOS 7 / 3.10.0-229.el7.x86_64

**_Use_**
- protobuf
- the client and the server on the same server
- 581 bytes payload
- 500/2000/5000 concurrent clients
- mock processing time: 0ms, 10ms and 30ms

**_Test Result_**

### mock 0ms process time

<table><tr><th>Throughputs</th><th>Mean Latency</th><th>P99 Latency</th></tr><tr><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p0-throughput.png"></td><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p0-latency.png"></td><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p0-p99.png"></td></tr></table>


### mock 10ms process time

<table><tr><th>Throughputs</th><th>Mean Latency</th><th>P99 Latency</th></tr><tr><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p10-throughput.png"></td><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p10-latency.png"></td><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p10-p99.png"></td></tr></table>


### mock 30ms process time

<table><tr><th>Throughputs</th><th>Mean Latency</th><th>P99 Latency</th></tr><tr><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p30-throughput.png"></td><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p30-latency.png"></td><td width="30%"><img src="http://colobu.com/2018/01/31/benchmark-2018-spring-of-popular-rpc-frameworks/p30-p99.png"></td></tr></table>


## Examples

You can find all examples at [rpcxio/rpcx-examples](https://github.com/rpcxio/rpcx-examples).

The below is a simple example.


**Server**

```go
    // define example.Arith
    ……

    s := server.NewServer()
	s.RegisterName("Arith", new(example.Arith), "")
	s.Serve("tcp", addr)

```


**Client**

```go
    // prepare requests
    ……

    d := client.NewPeer2PeerDiscovery("tcp@"+addr, "")
	xclient := client.NewXClient("Arith", client.Failtry, client.RandomSelect, d, client.DefaultOption)
	defer xclient.Close()
	err := xclient.Call(context.Background(), "Mul", args, reply, nil)
```

## Contribute

see [contributors](https://github.com/smallnest/rpcx/graphs/contributors).

Welcome to contribute:
- submit issues or requirements
- send PRs
- write projects to use rpcx
- write tutorials or articles to introduce rpcx

## License

Apache License, Version 2.0 
