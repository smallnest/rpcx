# PRD: RDMA Conn Transport (replace rsocket with rdmanet.Conn)

## Introduction

rpcx is a Go RPC framework that selects a transport via a "network" string (tcp, http, kcp, quic, unix, memu, iouring). It already has an experimental `rdma` transport gated behind the `rdma` Go build tag, but it is built on the third-party `github.com/smallnest/rsocket` package.

This feature replaces the `rsocket`-based implementation with one built on `rdmanet.Conn` (from `github.com/smallnest/gordma/rdmanet`). Unlike the lower-level `RawConn`, `rdmanet.Conn` already implements `Read`/`Write`/`Close` with built-in message framing and credit-based flow control — it is essentially an `io.ReadWriteCloser` over RDMA. The only gaps versus Go's `net.Conn` interface are that its `LocalAddr`/`RemoteAddr` return `string` (not `net.Addr`), and it has no `SetDeadline` family. So this feature adds a **thin adapter** that wraps `rdmanet.Conn` to satisfy `net.Conn`, plus a listener adapter over `rdmanet.Listener`, letting the rest of rpcx (codec, protocol, byte-stream) work unchanged.

All RDMA code stays behind the existing `rdma` build tag, so default builds and CI on non-RDMA hosts are unaffected.

## Goals

- Replace `rsocket` usage in `server/listener_rdma.go` and `client/connection_rdma.go` with `rdmanet.Conn`/`rdmanet.Listener`.
- Provide a thin `net.Conn` adapter over `rdmanet.Conn` (pass-through Read/Write/Close, plus `net.Addr` wrappers and a `SetDeadline` family).
- Provide a `net.Listener` adapter over `rdmanet.Listener` whose `Accept()` returns the `net.Conn` adapter.
- Keep the `rdma` build tag as the on/off switch.
- Use `host:port` addressing (the TCP out-of-band handshake address `rdmanet.Conn` expects).
- Treat `SetDeadline`/`SetReadDeadline`/`SetWriteDeadline` as a documented no-op (return nil, not enforced).
- Fully remove the `github.com/smallnest/rsocket` dependency from `go.mod`/`go.sum`; add `github.com/smallnest/gordma`.

## User Stories

### US-001: Add net.Conn adapter over rdmanet.Conn
**Description:** As an rpcx developer, I want a type that wraps `rdmanet.Conn` and implements `net.Conn`, so existing client/server byte-stream code can use RDMA without changes.

**Acceptance Criteria:**
- [ ] New file (e.g. `share/rdma_conn.go`) under `//go:build rdma`, defining a struct holding a `*rdmanet.Conn`.
- [ ] `Read`/`Write`/`Close` delegate directly to the embedded `*rdmanet.Conn` (which already frames messages and does flow control).
- [ ] `LocalAddr()`/`RemoteAddr()` return a non-nil `net.Addr` wrapping the `string` from `rdmanet.Conn.LocalAddr()`/`RemoteAddr()`, with `Network()` returning `"rdma"`.
- [ ] `SetDeadline`/`SetReadDeadline`/`SetWriteDeadline` return nil as a documented no-op (comment explains it is not enforced).
- [ ] A `var _ net.Conn = (*adapter)(nil)` assertion compiles.
- [ ] `go build -tags rdma ./...` succeeds (on a libibverbs Linux host; see Open Questions).

### US-002: Replace client rsocket dial with rdmanet.Dial
**Description:** As an rpcx user, I want `client/connection_rdma.go` to dial over `rdmanet.Conn` so RDMA client connections no longer depend on rsocket.

**Acceptance Criteria:**
- [ ] `newRDMAConn` calls `rdmanet.Dial(address, opts...)` or `rdmanet.DialTimeout(address, timeout, opts...)` instead of `rsocket.DialTCP`.
- [ ] The returned `*rdmanet.Conn` is wrapped in the US-001 adapter and returned as a `net.Conn`.
- [ ] `network != "rdma"` still returns an error as before.
- [ ] No reference to `rsocket` remains in the file.
- [ ] File keeps the `//go:build rdma` tag.

### US-003: Replace server rsocket listener with rdmanet.Listen
**Description:** As an rpcx user, I want `server/listener_rdma.go` to listen over `rdmanet.Listener` so RDMA server listeners no longer depend on rsocket.

**Acceptance Criteria:**
- [ ] `rdmaMakeListener` calls `rdmanet.Listen(address, opts...)` instead of `rsocket.NewTCPListener`.
- [ ] Returns a `net.Listener` adapter (US-004) wrapping the `*rdmanet.Listener`.
- [ ] Address parsing accepts `host:port` (no regression vs current `net.SplitHostPort`).
- [ ] `RDMA_BACKLOG` env handling is preserved or mapped to an equivalent listener option.
- [ ] No reference to `rsocket` remains in the file.
- [ ] File keeps the `//go:build rdma` tag.

### US-004: Add net.Listener adapter over rdmanet.Listener
**Description:** As an rpcx developer, I want a type that wraps `rdmanet.Listener` and implements `net.Listener` so `Accept()` yields the `net.Conn` adapter.

**Acceptance Criteria:**
- [ ] Struct holds a `*rdmanet.Listener`; implements `Accept() (net.Conn, error)`, `Close() error`, `Addr() net.Addr`.
- [ ] `Accept()` calls `rdmanet.Listener.Accept()` (returns `*rdmanet.Conn`) and wraps it in the US-001 adapter.
- [ ] `Addr()` wraps `rdmanet.Listener.Addr()` (a `string`) in a `net.Addr` with `Network()` returning `"rdma"`.
- [ ] `Close()` delegates to `rdmanet.Listener.Close()`.
- [ ] `var _ net.Listener = (*adapter)(nil)` compiles.

### US-005: Remove rsocket dependency
**Description:** As a maintainer, I want rsocket fully removed and gordma added so the dependency graph is clean.

**Acceptance Criteria:**
- [ ] No `rsocket` import remains (`grep -r rsocket --include=*.go .` returns nothing).
- [ ] `github.com/smallnest/rsocket` removed from `go.mod`; `github.com/smallnest/gordma` added.
- [ ] `go mod tidy` runs clean; `go.sum` updated; rsocket entries gone.
- [ ] Default (non-rdma) build `go build ./...` still passes.

## Functional Requirements

- FR-1: The system must provide a `net.Conn` adapter wrapping `*rdmanet.Conn`, compiled only under the `rdma` build tag.
- FR-2: The adapter's `Read`/`Write`/`Close` must delegate to the embedded `rdmanet.Conn` without re-framing.
- FR-3: The adapter must implement `LocalAddr`/`RemoteAddr` returning a non-nil `net.Addr` (network `"rdma"`) wrapping the underlying string addresses.
- FR-4: The adapter's `SetDeadline`/`SetReadDeadline`/`SetWriteDeadline` must return nil as a documented no-op.
- FR-5: The client factory `newRDMAConn` must establish connections via `rdmanet.Dial`/`rdmanet.DialTimeout` and return the US-001 adapter.
- FR-6: The server factory `rdmaMakeListener` must create listeners via `rdmanet.Listen` and return the US-004 adapter.
- FR-7: The system must accept `host:port` addresses for the `rdma` network.
- FR-8: The system must remove all `github.com/smallnest/rsocket` imports and the `go.mod`/`go.sum` entries, adding `github.com/smallnest/gordma`.
- FR-9: The system must keep all RDMA-specific code behind the `//go:build rdma` build tag.

## Non-Goals (Out of Scope)

- No use of `rdmanet.RawConn` (the lower-level post/poll endpoint); this feature uses the higher-level `Conn` per requirement.
- No manual framing, batching, or flow control in rpcx — `rdmanet.Conn` already provides these.
- No one-sided RDMA Write/Read verbs in the RPC data path.
- No changes to rpcx's protocol, codec, or service-discovery layers.
- No real (enforced) deadline support — explicitly a no-op in v1.
- No Windows/non-Linux RDMA support beyond what `rdmanet` already stubs.

## Technical Considerations

- `rdmanet.Conn` already implements `Read(p []byte) (int, error)`, `Write(p []byte) (int, error)`, `Close() error`, and message helpers (`SendMsg`/`RecvMsg`/`SendBatch`...). The adapter only bridges the `net.Conn` gaps: `LocalAddr`/`RemoteAddr` type (`string` → `net.Addr`) and the missing `SetDeadline` family.
- `rdmanet.Listener.Accept()` returns `*rdmanet.Conn`; `Addr()` returns `string` — both need wrapping for `net.Listener`.
- Constructors: `Dial(addr string, opts ...Option)`, `DialTimeout(addr, timeout, opts...)`, `Listen(addr string, opts ...Option)`. Options include `WithDevice`, `WithPort`, `WithGIDIndex`, `WithQueueDepth`, `WithBufferSize`, `WithHandshake` (these apply to `Conn`, unlike `RawConn` which ignores some).
- gordma's Linux path uses cgo + libibverbs; the stub build returns `gordma.ErrNotSupported`. Real build/test needs a Linux host with libibverbs.
- rpcx wiring: `client.ConnFactories["rdma"]` and `server.makeListeners["rdma"]` registered in `init()` — keep this pattern.
- Module path: `github.com/smallnest/gordma`, subpackage `rdmanet`.

## Success Metrics

- `grep -r rsocket --include=*.go .` returns zero matches.
- `go build ./...` (default tags) passes.
- `go build -tags rdma ./...` compiles on a libibverbs Linux host.
- An rpcx client can call an rpcx server over the `rdma` network with the same RPC semantics as over `tcp`.

## Open Questions

- Can the real `-tags rdma` build/end-to-end test run in this environment, or only on a separate RDMA-capable Linux host?
- Should `WithBufferSize`/`WithQueueDepth` be exposed via env vars (like `RDMA_BACKLOG`) or left at `rdmanet` defaults for v1? (Recommend defaults for v1, since `Conn` manages buffers internally.)
- Preferred adapter location: single `share/rdma_conn.go` reused by client/server, vs duplicated. (Recommend single shared file.)
