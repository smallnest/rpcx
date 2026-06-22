# PRD: 方法级服务注册 — 注册 struct 时指定只暴露哪些方法

GitHub Issue: #581
Status: Draft

## Introduction / 概述

rpcx 服务端通过 `Register(rcvr, metadata)` / `RegisterName(name, rcvr, metadata)` 注册一个 struct，框架用反射扫描它**所有签名合适的导出方法**，把它们**全部**变成可被远程调用的 RPC 端点（见 `server/service.go:172` 调用 `suitableMethods`）。

问题是：导出方法不等于"想对外暴露的 RPC"。一个 service struct 上常常有些导出方法是给同包/同进程其他代码用的（比如供 HTTP handler 或 jsonrpc 复用的业务方法），开发者并不想把它们也开成 RPC 端点。今天做不到——要么把方法改成非导出（同包代码就调不到了），要么把 struct 拆开。Issue #581 的诉求正是："注册时能指定具体注册哪些成员方法"，**避免暴露不该暴露的方法**。rpcx 作者已在 issue 中确认该诉求，并归入 `8.0`。

本特性新增一组**白名单式**的注册入口：调用方显式列出要暴露的方法名，只有白名单内的方法被注册为 RPC，其余导出方法一律不暴露。现有 `Register`/`RegisterName` 签名与行为**完全不变**。

## Goals / 目标

- 提供注册入口，让调用方显式指定一个 struct 上**只**注册哪些成员方法。
- 采用白名单（默认不暴露），契合"避免误暴露"的安全初衷。
- 白名单中出现不存在或签名不合适的方法名时**报错**，避免拼写错误导致接口静默缺失。
- 不改动现有 `Register`/`RegisterName` 的签名与行为——纯增量，零破坏。

## User Stories

### US-001: 新增白名单注册入口 RegisterWithMethods
**Description:** 作为 rpcx 服务开发者，我想在注册 struct 时只暴露指定的几个方法，这样其余导出方法不会变成 RPC 端点。

**Acceptance Criteria:**
- [ ] 新增 `func (s *Server) RegisterWithMethods(rcvr interface{}, methods []string, metadata string) error`
- [ ] 新增 `func (s *Server) RegisterNameWithMethods(name string, rcvr interface{}, methods []string, metadata string) error`
- [ ] 注册后，该 service 的 `service.method` 只包含白名单列出的方法
- [ ] 白名单外的导出方法不可被远程调用（调用返回方法不存在错误）
- [ ] 现有 `Register`/`RegisterName` 签名与行为不变
- [ ] `go build ./...`、`go vet ./...` 通过

### US-002: 白名单校验 — 不存在或签名不合适的方法名报错
**Description:** 作为服务开发者，当我把方法名拼错或列入一个签名不符合 RPC 要求的方法时，我希望注册立刻报错，而不是静默少注册一个接口。

**Acceptance Criteria:**
- [ ] 白名单中某方法名在该 struct 上不存在（非导出/根本没有）→ 返回错误，错误信息包含该方法名
- [ ] 白名单中某方法存在但签名不满足 RPC 要求（非 `suitableMethods` 认可的形态）→ 返回错误，错误信息包含该方法名
- [ ] 报错时该 service **不**被注册（不出现部分注册的中间状态）
- [ ] 有针对上述两种错误的单元测试，断言返回 error 且 service 未注册
- [ ] `go test ./server/` 通过

### US-003: 空白名单语义 — 视为未指定并报错
**Description:** 作为服务开发者，当我传了 nil 或空的方法列表时，我希望得到明确的错误提示，而不是意外地注册了全部或一个都没注册。

**Acceptance Criteria:**
- [ ] `methods` 为 nil 或长度为 0 时，返回错误，提示应使用 `Register`/`RegisterName` 注册全部方法
- [ ] 此时该 service 不被注册
- [ ] 有单元测试覆盖 nil 和空切片两种输入
- [ ] `go test ./server/` 通过

### US-004: 文档与示例
**Description:** 作为 rpcx 用户，我想从文档知道如何只暴露部分方法，以及白名单的校验规则。

**Acceptance Criteria:**
- [ ] 在 README 或 server 包文档中说明 `RegisterWithMethods`/`RegisterNameWithMethods` 的用法
- [ ] 文档写明：白名单式、不存在/签名不符报错、空白名单报错三条规则
- [ ] 提供一个最小代码示例（注册 3 个方法中的 2 个）
- [ ] `go vet ./...` 通过

## Functional Requirements

- FR-1: The system must provide `RegisterWithMethods(rcvr, methods []string, metadata string) error`.
- FR-2: The system must provide `RegisterNameWithMethods(name string, rcvr, methods []string, metadata string) error`.
- FR-3: The system must register only the methods named in the whitelist, excluding all other exported methods of the struct.
- FR-4: The system must return an error naming any whitelist entry that does not exist as an exported method on the struct.
- FR-5: The system must return an error naming any whitelist entry whose signature is not a suitable RPC method.
- FR-6: The system must return an error and register nothing when the whitelist is nil or empty.
- FR-7: The system must not register the service when any validation error occurs (no partial registration).
- FR-8: The system must leave the existing `Register` and `RegisterName` signatures and behavior unchanged.

## Non-Goals (Out of Scope)

- 不支持黑名单（"排除某些方法"）；只做白名单。
- 不改 `Register`/`RegisterName` 的签名（不引入可变参数或 functional option）。
- 不做方法级的访问控制/鉴权（白名单只决定"是否注册为 RPC"，不涉及调用时的权限）。
- 不支持运行时动态增删已注册 service 的方法（注册即固定）。
- 不统一 HTTP handler / jsonrpc 的导出模型（issue 提到的跨协议统一愿景超出本特性范围）。
- 不改 `RegisterFunction`/`RegisterFunctionName`（函数级注册本就是单个，不需要白名单）。

## Design Considerations

- 现有 `register`（`server/service.go:148`）写死 `service.method = suitableMethods(service.typ, true)`，把全部合适方法注册。白名单版本应在拿到 `suitableMethods` 结果后按白名单**过滤**，并对白名单里"没出现在结果中"的名字做校验分流（不存在 vs 签名不符）。
- `suitableMethods`（`service.go:271`）已按方法名建 `map[string]*methodType`，过滤与校验都可基于这张 map 完成；判断"存在但签名不符"可借助 `typ.MethodByName` 区分。
- 建议 `register`/`registerName` 内部抽出一个共用的核心，新旧入口都走它，差别仅在是否传入白名单，避免逻辑重复。

## Technical Considerations

- 改动集中在 `server/service.go`，无新增依赖。
- 错误信息需可定位：包含 service 名与具体方法名。
- 并发安全沿用现有 `serviceMapMu` 锁，不引入新的并发模型。

## Success Metrics

- 调用方能用一行 `RegisterWithMethods` 把 N 个导出方法中的 M 个暴露为 RPC，其余 N−M 个无法被远程调用。
- 拼错方法名时注册必定报错（不会静默少接口）。
- 现有 server 测试零回归。

## Open Questions

- 方法名匹配是否需要大小写不敏感？（当前定：精确匹配。）
- 是否需要一个返回"该 struct 上所有可注册方法名"的辅助函数，方便用户构造白名单？（可作为后续增强。）
- `RegisterFunction` 系列是否也想要类似能力？（当前定：不在范围。）
