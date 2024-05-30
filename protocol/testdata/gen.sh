# curl -O https://raw.githubusercontent.com/rpcxio/rpcx-benchmark/master/proto/benchmark.proto

# generate .go files from IDL

protoc -I.  --go_out=. --go_opt=module="testdata"   ./benchmark.proto