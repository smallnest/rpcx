# curl -O https://raw.githubusercontent.com/rpcxio/rpcx-benchmark/master/proto/benchmark.proto

# generate .go files from IDL
protoc --go_out=./ ./benchmark.proto

