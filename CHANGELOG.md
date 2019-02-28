# [rpcx](http://rpcx.site)

## 5.0 (developing)

- support jsonrpc 2.0
- support opentracing and opencensus
- upload/download files by streaming

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
- Setup rpcx offcial site: http://rpcx.site
- Add Chinese document: http://doc.rpcx.site or https://smallnest.gitbooks.io/go-rpc-programming-guide

## 3.1

- Add http gateway: https://github.com/rpcx-ecosystem/rpcx-gateway
- Add direct http invoke
- Add bidirectional communication 
- Add xgen tool to generate codes for services automatically


fix bugs.

## 3.0

- Rewrite rpcx. It implements its protocol and won't implemented based on wrapper of go standard rpc lib
- Add go tags for pluggable plugins
- Add English document: https://github.com/smallnest/rpcx-programming
- Add rpcx 3.0 examples: https://github.com/rpcx-ecosystem/rpcx-examples3

rpcx 3.0 is not compatible with rpcx 2.0 and below