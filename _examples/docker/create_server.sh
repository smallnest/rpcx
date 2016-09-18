GOOS=linux go build -a -installsuffix cgo -o server server.go

docker rmi -f rpcx-server
docker build -t rpcx-server  .
docker run -it -p 127.0.0.1:8972:8972 -p 127.0.0.1:8973:8973 rpcx-server