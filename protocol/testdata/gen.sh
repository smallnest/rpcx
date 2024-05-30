# curl -O https://raw.githubusercontent.com/rpcxio/rpcx-benchmark/master/proto/benchmark.proto

# generate .go files from IDL

protoc -I.  --go_out=. --go_opt=module="github.com/smallnest/rpcx/protocol/testdata"   ./benchmark.proto