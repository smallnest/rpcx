# xgen

`xgen` isn a tool that can help you generate a server stub for rpcx services.

It search structs in your specified files and add them as services. Currently it doesn't support registring functions.

## Usage

```sh
# install
go get -u github.com/smallnest/rpcx/tool/xgen/...

# run
xgen -o server.go <file>.go
```

The above will generate server.go containing a rpcx which registers all exported struct types contained in `<file>.go`.


## Options

```
  -o string
    	specify the filename of the output
  -pkg
    	process the whole package instead of just the given file
  -r string
    	registry type. support etcd, consul, zookeeper, mdns (default "etcd")
  -tags string
    	build tags to add to generated file
```

You can run as:

```sh
xgen [options] <file1>.go <file2>.go <file3>.go 
```

for example, `xgen -o server.go a.go b.go /User/abc/go/src/github.com/abc/aaa/c.go`

or

```sh
xgen [options] <package1> <package1> <package1>
```

for example, `xgen -o server.go github.com/abc/aaa github.com/abc/bbb github.com/abc/ccc`