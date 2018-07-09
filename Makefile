WORKDIR=`pwd`

default: build

vet:
	go vet ./...

tools:
	go get honnef.co/go/tools/cmd/staticcheck
	go get honnef.co/go/tools/cmd/gosimple
	go get honnef.co/go/tools/cmd/unused
	go get github.com/gordonklaus/ineffassign
	go get github.com/fzipp/gocyclo
	go get github.com/golang/lint/golint
	go get github.com/alexkohler/prealloc

gometalinter:
	gometalinter --enable-all ./...
lint:
	golint ./...

staticcheck:
	staticcheck -ignore "$(shell cat .checkignore)" ./...

gosimple:
	gosimple -ignore "$(shell cat .gosimpleignore)" ./...

unused:
	unused ./...

ineffassign:
	ineffassign .

gocyclo:
	gocyclo -over 20 $(shell find . -name "*.go" |egrep -v "_testutils/*|vendor/*|pb\.go|_test\.go")

prealloc:
	prealloc ./...

check: staticcheck gosimple ineffassign

doc:
	godoc -http=:6060

deps:
	go list -f '{{ join .Deps  "\n"}}' ./... |grep "/" | grep -v "github.com/smallnest/rpcx"| grep "\." | sort |uniq

fmt:
	go fmt ./...

build:
	go build ./...

build-all:
	go build -tags "reuseport kcp quic zookeeper etcd consul ping utp rudp" ./...

test:
	go test -race -tags "reuseport kcp quic zookeeper etcd consul ping utp rudp" ./...

glide-mirror:
	@glide mirror set https://golang.org/x/net https://github.com/golang/net
	@glide mirror set https://golang.org/x/tools https://github.com/golang/tools
	@glide mirror set https://golang.org/x/text https://github.com/golang/text
	@glide mirror set https://golang.org/x/exp https://github.com/golang/exp
	@glide mirror set https://golang.org/x/image https://github.com/golang/image
	@glide mirror set https://golang.org/x/sys https://github.com/golang/sys
	@glide mirror set https://golang.org/x/crypto https://github.com/golang/crypto
	@glide mirror set https://golang.org/x/sync https://github.com/golang/sync
	@glide mirror set https://golang.org/x/time https://github.com/golang/time
	@glide mirror set https://golang.org/x/oauth2 https://github.com/golang/oauth2

	
