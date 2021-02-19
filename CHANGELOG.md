# [rpcx](http://rpcx.io)

## 7.0 (developing)
- move etcd support to github.com/rpcxio/rpcx-etcd
- Broken API: NewXXXDiscovery returns error instead of panic
- support AdvertiseAddr in FileTransfer
- support Auth for OneClientPool
- support Auth for XClientPool
- Broken API: add meta parameter for SendFile/DownloadFile 
- support streaming between server side and client side
- support DNS as service discovery

## 6.0 

- support reflection
- add kubernetes config example
- improve nacos support
- improve message.Encode performance
- re-register services in etcd v3
- avoid duplicated client creation
- add SelectNodePlugin that can interrupt the Select method
- support TcpCopy by TeePlugin
- support reuseport for http invoke
- return reply even in case of server errors
- Change two methods' name of client plugin!
- Broken API: add error parameter in `PreWriteResponse`(#486)
- Broken API: change ReadTimeout/WriteTimeout to IdleTimeout
- Support passing Deadline of client contexts to server side
- remove InprocessClient plugin
- use heartbeat/tcp_keepalive to avoid client hanging


## 5.0 

- support jsonrpc 2.0
- support CORS for jsonrpc 2.0
- support opentracing and opencensus
- upload/download files by streaming
- add Pool for XClient and OneClient
- remove rudp support
- add ConnCreated plugin. Yu can use it to set KCP UDPSession
- update client plugins. All plugin returns error instead of bool
- support ETCD 3.0 API
- support redis as registry
- support redis DB selection
- fix RegisterFunction issues
- add Filter for clients
- remove most of build tags such as etcd, zookeeper,consul,reuseport
- add Nacos as registry http://nacos.io
- support blacklist and whitlist

## 4.0

- Support utp and rudp
- Add OneClient to support invoke multile servicesby one client
- Finish compress feature
- Add more plugins for monitoring connection
- Support dynamic port allocation
- Use go module to manage dependencies
- Support shutdown graceful
- Add [rpcx-java](https://github.com/smallnest/rpcx-java) to support develop raw java services and clients
- Support thrift codec 
- Setup rpcx offcial site: http://rpcx.io
- Add Chinese document: http://cn.doc.rpcx.io or https://smallnest.gitbooks.io/go-rpc-programming-guide

## 3.1

- Add http gateway: https://github.com/rpcxio/rpcx-gateway
- Add direct http invoke
- Add bidirectional communication 
- Add xgen tool to generate codes for services automatically


fix bugs.

## 3.0

- Rewrite rpcx. It implements its protocol and won't implemented based on wrapper of go standard rpc lib
- Add go tags for pluggable plugins
- Add English document: https://github.com/smallnest/rpcx-programming
- Add rpcx 3.0 examples: https://github.com/rpcxio/rpcx-examples

rpcx 3.0 is not compatible with rpcx 2.0 and below