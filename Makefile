WORKDIR=`pwd`

default: build

vet:
	go vet ./...

tools:
	go get github.com/golangci/golangci-lint/cmd/golangci-lint
	go get github.com/golang/lint/golint
	go get github.com/axw/gocov/gocov
	go get github.com/matm/gocov-html

golangci-lint:
	golangci-lint run -D errcheck --build-tags 'quic kcp'

lint:
	golint ./...

doc:
	godoc -http=:6060

deps:
	go list -f '{{ join .Deps  "\n"}}' ./... |grep "/" | grep -v "github.com/smallnest/rpcx"| grep "\." | sort |uniq

fmt:
	go fmt ./...

build:
	go build ./...

build-all:
	go build -tags "kcp quic" ./...

test:
	go test -race -tags "kcp quic" ./...

cover:
	gocov test -tags "kcp quic" ./... | gocov-html > cover.html
	open cover.html

check-libs:
	GIT_TERMINAL_PROMPT=1 GO111MODULE=on go list -m -u all | column -t

update-libs:
	GIT_TERMINAL_PROMPT=1 GO111MODULE=on go get -u -v ./...

mod-tidy:
	GIT_TERMINAL_PROMPT=1 GO111MODULE=on go mod tidy
