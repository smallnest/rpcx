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
	go get github.com/axw/gocov/gocov
	go get -u gopkg.in/matm/v1/gocov-html

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
	go build -tags "reuseport kcp quic zookeeper etcd consul ping utp" ./...

test:
	go test -race -tags "reuseport kcp quic zookeeper etcd consul ping utp" ./...

cover:
	gocov test ./... | gocov-html > cover.html
	open cover.html

check-libs:
	GIT_TERMINAL_PROMPT=1 GO111MODULE=on go list -m -u all | column -t

update-libs:
	GIT_TERMINAL_PROMPT=1 GO111MODULE=on go get -u -v ./...

mod-tidy:
	GIT_TERMINAL_PROMPT=1 GO111MODULE=on go mod tidy
