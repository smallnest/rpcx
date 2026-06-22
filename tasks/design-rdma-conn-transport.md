Title: 用 rdmanet.Conn 接 rpcx 的 RDMA 传输——薄薄一层适配器替掉 rsocket

Author(s): chaoyuepan

Last updated: 2026-06-22

Discussion at: tasks/prd-rdma-conn-transport.md

Status: Draft

## Abstract / 摘要

我们把 rpcx 实验性的 `rdma` 传输从第三方 `github.com/smallnest/rsocket` 切到 `github.com/smallnest/gordma/rdmanet` 的 `Conn`。和裸端点 `RawConn` 不同，`rdmanet.Conn` 已经把分帧、信用流控、托管缓冲都做好了，自带 `Read/Write/Close`——它就是一条 RDMA 上的 `io.ReadWriteCloser`。它离 `net.Conn` 只差两样东西：`LocalAddr/RemoteAddr` 返回的是 `string` 而不是 `net.Addr`，以及没有 `SetDeadline` 一族。所以我们只写一层**很薄的**适配器补这两个洞，rpcx 的协议、编解码、字节流一行都不用动。

全部代码待在已有的 `//go:build rdma` 标签后面。**最重要的承诺：不开 `rdma` 标签时，默认构建产物与 rpcx 公开 API 一个字节都不变。** 这是硬约束，其余取舍都让位于它。

## Background / 背景与动机

rpcx 用一个 "network" 字符串选传输，注册表是两张 map：客户端 `client.ConnFactories[network]` 返回 `net.Conn`，服务端 `server.makeListeners[network]` 返回 `net.Listener`。tcp/http/kcp/quic/unix/memu/iouring 都按这契约接进来，现有的 `rdma` 也是——只不过它绑死在 rsocket 上：

```go
// client/connection_rdma.go —— 现状
return rsocket.DialTCP(address)            // rsocket 直接给一个 net.Conn

// server/listener_rdma.go —— 现状
return rsocket.NewTCPListener(host, p, blog) // rsocket 直接给一个 net.Listener
```

rsocket 把 `net.Conn`/`net.Listener` 都替我们实现了，接入只要两行。代价是整条 RDMA 数据路径锁死在一个我们不维护、演进不受控的第三方库上。`gordma` 是我们自己的 RDMA 栈，把 rpcx 接到它上面，我们才掌控这条路径。

这次的痛点和 RawConn 那版不同——`rdmanet.Conn` 已经是 `io.ReadWriteCloser` 了，分帧和流控都不缺。差的只是 `net.Conn` 接口的几个签名：

```go
// rdmanet.Conn 有这些（已经够用）：
func (c *Conn) Read(p []byte) (int, error)
func (c *Conn) Write(p []byte) (int, error)
func (c *Conn) Close() error
func (c *Conn) LocalAddr() string    // 注意：string，不是 net.Addr
func (c *Conn) RemoteAddr() string   // 注意：string，不是 net.Addr
// 没有 SetDeadline / SetReadDeadline / SetWriteDeadline

// rdmanet.Listener.Accept() 返回 *rdmanet.Conn（不是 net.Conn）；Addr() 返回 string。
```

所以这层适配器的活儿，**只是把类型对齐**：`string→net.Addr`、补三个 no-op 的 deadline 方法、把 `Accept` 的返回值再包一下。Read/Write/Close 直接转发就行。换句话说，rsocket 留下的洞，这次几乎能原样填回去，唯一要新写的是"接口形状"这一薄层。

## Design / 设计

### 一句话：transparent 转发，只补 net.Conn 缺的两块

`rdmanet.Conn` 的字节流语义已经和 `net.Conn` 对齐了，所以适配器不重新分帧、不重新缓冲，纯转发。它只负责两件事：把 `string` 地址包成 `net.Addr`，把不存在的 deadline 做成 no-op。

#### Conn 适配器

```go
//go:build rdma

type rdmaConn struct {
	*rdmanet.Conn   // 内嵌：Read/Write/Close 自动透传
}

func (c *rdmaConn) LocalAddr() net.Addr  { return rdmaAddr(c.Conn.LocalAddr()) }
func (c *rdmaConn) RemoteAddr() net.Addr { return rdmaAddr(c.Conn.RemoteAddr()) }

// deadline 三件套：rdmanet.Conn 没有原生 deadline，统一做成 no-op。
func (c *rdmaConn) SetDeadline(t time.Time) error      { return nil }
func (c *rdmaConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *rdmaConn) SetWriteDeadline(t time.Time) error { return nil }

var _ net.Conn = (*rdmaConn)(nil)

// rdmaAddr 把 rdmanet 的字符串地址包成 net.Addr，Network() 恒为 "rdma"。
type rdmaAddr string
func (a rdmaAddr) Network() string { return "rdma" }
func (a rdmaAddr) String() string  { return string(a) }
```

边界：`Read/Write/Close` 靠内嵌 `*rdmanet.Conn` 自动透传，适配器**不碰**数据路径。这是它和 RawConn 适配器最大的区别——那边要自己写长度前缀和残留缓冲，这边一行都不写。

#### Listener 适配器

```go
type rdmaListener struct{ l *rdmanet.Listener }

func (a *rdmaListener) Accept() (net.Conn, error) {
	c, err := a.l.Accept()              // 返回 *rdmanet.Conn
	if err != nil { return nil, err }
	return &rdmaConn{Conn: c}, nil
}
func (a *rdmaListener) Addr() net.Addr { return rdmaAddr(a.l.Addr()) } // string→net.Addr
func (a *rdmaListener) Close() error   { return a.l.Close() }

var _ net.Listener = (*rdmaListener)(nil)
```

#### 改造前 vs 改造后

```go
// 客户端：改造后
func newRDMAConn(c *Client, network, address string) (net.Conn, error) {
	if network != "rdma" {
		return nil, errors.New("network is not rdma")
	}
	conn, err := rdmanet.DialTimeout(address, c.option.ConnectTimeout)
	if err != nil { return nil, err }
	return &rdmaConn{Conn: conn}, nil
}

// 服务端：改造后
func rdmaMakeListener(s *Server, address string) (net.Listener, error) {
	host, port, _ := net.SplitHostPort(address)        // 仍是 host:port
	l, err := rdmanet.Listen(net.JoinHostPort(host, port))
	if err != nil { return nil, err }
	return &rdmaListener{l: l}, nil
}
```

`ConnFactories["rdma"]` / `makeListeners["rdma"]` 的 `init()` 注册原样不动：接入点没变，只换了返回值的来源。

#### 地址与参数

地址沿用 `host:port`（`rdmanet.Conn` 做 TCP 带外握手用的地址）。`rdmanet.Conn` 自己管理缓冲，v1 用 `rdmanet` 默认参数即可，不急着引入一堆环境变量；`RDMA_BACKLOG` 这类如确有需要可后置（见 Open Questions）。

### Deadline：诚实地做成 no-op

`rdmanet.Conn` 没有原生 deadline。我们让 `SetDeadline` 一族**返回 nil 但不生效**，注释写明是已知 no-op。理由见 Rationale。

## Rationale / 理由与取舍

### 为什么用 rdmanet.Conn，而不是 RawConn

这是本设计最关键的一个岔路，也有一篇姊妹设计文档专门论证 RawConn 那条路（见 `design-rdma-rawconn-transport.md`）。两者的取舍很清楚：

- **`Conn` 自带分帧 + 信用流控 + 托管缓冲**，适配器只补类型形状，几十行封顶，正确性几乎不用论证——数据路径根本没经过我们的代码。
- **`RawConn` 是裸端点**，要我们在适配器里手写长度前缀分帧、残留缓冲、post/poll 驱动，换来的是未来做 batch/pipeline/单边 RDMA 的吞吐天花板。

我们这版选 `Conn`，因为目标是"干净地替掉 rsocket、让 rpcx 跑在自家 RDMA 栈上"，**正确和薄**比"榨干线速"更重要。代价是：`Conn` 的托管层会把极限吞吐优化的空间提前焊死——真要打满线速，得回到 RawConn 那条路。这个代价我们认，并且把 RawConn 方案完整保留为另一篇文档，谁需要谁去取。

### 为什么是"适配器"，而不是改 rpcx 的传输抽象

最朴素的做法是让 rpcx 协议层直接认识 `rdmanet.Conn`。我们没选，因为它要侵入核心读写循环、破坏 `net.Conn` 这层统一抽象，只为一个实验传输买单。适配器把 RDMA 特异性关在一个文件里，上层零改动——直接服务于"默认构建不变"的硬承诺。何况这次适配器薄到几乎只有类型转换，侵入式改造更没有理由。

### 为什么 deadline 做成 no-op 而不是报错

三个选项：报 `not supported` 错、给读写加尽力而为的超时、或静默 no-op。rpcx 调用路径会主动 `SetDeadline`——返回错误会直接打断正常 RPC 流程；硬塞超时又是在 v1 引入一套没人验证过的半成品语义。**我们选静默 no-op：返回 nil、不生效、注释写清。** 宁可诚实地"暂不支持"，也不假装支持。

### 为什么彻底删掉 rsocket，而不是并存

留着 rsocket 当备选，等于让模块多扛一个不维护的依赖、多一条没人走的代码路径。既然 `rdmanet.Conn` 全面接管 RDMA 路径，就把 rsocket 从 `go.mod`/`go.sum` 连根拔掉，`grep -r rsocket` 必须零命中。

## Compatibility / 兼容性

**对默认用户，这不是破坏性变更。** 全部改动在 `//go:build rdma` 后面；不开标签的人构建产物逐字节不变，rpcx 公开 API 不变。这是开门见山的承诺，也是验收项（`go build ./...` 默认标签必须过）。

对**已经在用 `-tags rdma` + rsocket 的用户**，这是破坏性变更，诚实列代价：

- **线缆不兼容**：`rdmanet.Conn` 的握手与分帧和 rsocket 不互通，两端必须同时升级，不能混部。
- **行为变化**：deadline 从"由 rsocket/TCP 实现"变成 no-op；依赖 RDMA 连接读写超时的代码需知晓。
- **依赖变化**：`go.mod` 移除 rsocket、新增 gordma；下游 `go mod tidy` 会看到依赖图变动。

迁移路径：`rdma` 标签本身就是特性开关，在 RDMA 主机上重新构建、两端同时切换即可；非 RDMA 用户无需任何动作。这是实验性传输（藏在 build tag 后），破坏面有限，不提供线缆层兼容垫片。

## Implementation / 实现与过渡

落地顺序，每步可独立验证：

1. **写 `net.Conn` 适配器**（`rdmaConn` + `rdmaAddr`）和 `net.Listener` 适配器（`rdmaListener`），全部 `//go:build rdma`。
2. **改客户端** `newRDMAConn` → `rdmanet.Dial`/`DialTimeout`，包成 `rdmaConn`。
3. **改服务端** `rdmaMakeListener` → `rdmanet.Listen`，包成 `rdmaListener`。
4. **删 rsocket**：移除所有 import、`go.mod`/`go.sum` 条目，加 gordma，`go mod tidy`。

可落地性靠两类证据，不靠"风险可控"的口号：

- **构建闸门**：`go build ./...`（默认）与 `go build -tags rdma ./...`（libibverbs Linux 主机）都过；`go vet -tags rdma ./...` 干净；`grep -r rsocket --include=*.go .` 零命中。
- **接口断言**：`var _ net.Conn = (*rdmaConn)(nil)` 和 `var _ net.Listener = (*rdmaListener)(nil)` 编译期就保证形状对齐——这是这版"薄适配器"最强的正确性保证，因为数据路径不经过我们的代码。

因为适配器不碰数据路径，逻辑层几乎没有可单测的东西；真正的验证落在真机端到端（rpcx client over `rdma` 调通 server），依赖 libibverbs 的 Linux 主机，见 Open Questions。

## Open Questions

- 真机 `-tags rdma` 端到端验证能否在现有环境完成，还是只能在独立 RDMA Linux 主机上做？
- 是否要把 `WithBufferSize`/`WithQueueDepth`/`RDMA_BACKLOG` 经环境变量透出，还是 v1 全用 `rdmanet` 默认？（倾向 v1 用默认，`Conn` 自管缓冲。）
- 适配器文件放 `share/rdma_conn.go` 供 client/server 复用，还是各自一份？（倾向单文件复用。）
- 与姊妹方案 RawConn 的关系：两版会并存于 `tasks/`，未来若要极限吞吐再切 RawConn；是否需要在 README 标注"默认 Conn、高性能选 RawConn"的取向？
