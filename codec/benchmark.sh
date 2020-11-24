# generate .go files from IDL
protoc --go_out=./ ./test_data/protobuf.proto

thrift -r -out ./ --gen go ./test_data/thrift_colorgroup.thrift

# # run benchmarks
# go test -bench=. -run=none

# # clean files
# rm -rf ./test_data/*.go