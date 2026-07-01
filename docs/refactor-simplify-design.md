# 代码简化设计文档 (Refactor & Simplify)

本文档记录对最近一轮重构（god-object 文件拆分 + 插件错误日志 + 方法白名单注册）
的**简化 review** 与落地改动。目标是**质量**，不涉及正确性 bug 修复。

## 1. 背景

最近的提交（`HEAD~11...HEAD`）主要做了两件事：

1. **文件拆分**：把两个"上帝对象"拆成职责内聚的小文件。
   - `client/xclient.go` (1212 行) → `xclient_broadcast.go` / `xclient_call.go`
     / `xclient_discovery.go` / `xclient_transfer.go`
   - `server/server.go` (799 行) → `server_conn.go` / `server_dispatch.go`
     / `server_response.go` / `server_shutdown.go`
2. **行为增强**：把之前 `_ = plugin.DoXxx(...)` 吞掉的错误改为记录日志；新增
   `RegisterWithMethods` 方法白名单注册；空白名单守卫。

拆分本身是**逐行搬移**（byte-for-byte），四位 reviewer 一致确认搬移无漂移。
真正需要简化的是**新增逻辑**。

## 2. 发现与处理

四个维度（Reuse / Simplification / Efficiency / Altitude）并行 review，去重后
落地如下改动。

### 2.1 已修复

| # | 文件 | 问题 | 简化后 |
|---|------|------|--------|
| 1 | `server/service.go` | **三重空白名单守卫**：`RegisterWithMethods`、`RegisterNameWithMethods` 各有一次 `len(methods)==0` 检查，`register()` 内部 `else` 分支又重复了第三次（且错误文案不同）。第三处对白名单路径不可达（两个公开入口已拦截）。 | 删除 `register()` 内的死分支，改为注释说明：空 slice 会自然落到下方"no suitable methods"检查。规则收敛，文案不再三份。 |
| 2 | `server/service.go` | **`RegisterWithMethods` 缺少 nil-Plugins 守卫**：`RegisterName`/`RegisterNameWithMethods` 在 `DoRegister` 前都有 `if s.Plugins == nil` 兜底，唯独 `RegisterWithMethods` 没有，与同族方法不一致（潜在 nil 解引用）。 | 补齐 `if s.Plugins == nil { s.Plugins = &pluginContainer{} }`，与同族方法对齐。 |
| 3 | `server/server_shutdown.go` | **拆分留下的接缝垃圾**：`Serve` 的文档注释被搬到 shutdown 文件却无对应函数（悬空注释）；`getDoneChan` 被搬来后全仓零调用（死代码）。 | 删除死函数 `getDoneChan` 与悬空注释；把 `Serve` 的文档注释还给 `server.go` 中 `Serve` 函数上方。 |
| 4 | `server/service.go` | `reflect.PtrTo` 已废弃（编译器 diagnostic）。 | 替换为 `reflect.PointerTo`。 |
| 5 | `server/plugin_test.go` | 新增的 `waitServerReady`（返回 bool，goroutine 安全）已用于消除 goroutine 内的 `t.Fatalf`，但 `TestPluginHeartbeat` 的子 goroutine 里还残留一处 `t.Fatalf`（`go vet` 报警）。 | 改为 `t.Errorf` + `return`，补全该模式，`go vet` 干净。 |

### 2.2 已评估但跳过（避免改变意图或超出 diff 范围）

- **插件错误日志"每调用点手写"而非收敛到 `pluginContainer`**（Altitude 提出）：
  reviewer 建议把"吞错并记日志 vs 返回错误"的策略下沉到 container 内部，让调用点
  统一 `_ =` 或统一返回。这是**合理的深层改进**，但涉及重新设计 `Do*` 方法族的
  错误契约，波及所有插件调用点，**远超本次 review 的 diff 范围**，且属于行为/接口
  变更（`/simplify` 明确排除）。留作后续独立重构项。当前每处日志的 message 各有
  上下文（`servicePath.method`），并非可折叠的 copy-paste。

- **方法白名单未复用/泛化 `suitableMethods`**（Altitude/Reuse 提出）：白名单路径
  自己重走 `all` 建 `picked`，并额外 `MethodByName` 探测以区分"不存在"与"签名不合适"。
  reviewer 建议给 `suitableMethods` 加过滤器参数统一两条路径。这会改动核心注册机制
  的签名与语义，属于设计层变更，**跳过**——现有实现正确且已复用 `suitableMethods`
  的产出，`MethodByName` 探测仅用于生成更友好的错误文案，代价可接受。

- **`nil` vs `len==0` 语义**：`register` 用 `methods == nil` 表示"注册全部"，非 nil
  表示白名单。这是刻意的哨兵语义，公开 API 已在边界拦住空 slice，改为显式 intent
  参数属于 API 变更，**跳过**。

- **doc-only / 注释冗长**（`share/context.go`、`client/discovery.go`）：均为文档
  措辞层面，不影响逻辑，**跳过**。

- **`xclient_discovery.go` `watch()` 去掉了 `sort.Slice`**：这是**减少**工作量而非
  回归，且属搬移期的既有行为变更（前序提交 `cd34c37` 已单独处理），**不在本次范围**。

## 3. 拆分边界评估（Altitude）

四位 reviewer 一致认为新文件边界**内聚合理**：

- Server 侧：`conn`（监听/连接/单请求读取/鉴权）、`dispatch`（请求分发/函数调用/错误）、
  `response`（发送响应）、`shutdown`（生命周期/优雅关闭/重启）——四个真实职责簇。
- Client 侧：`broadcast`（Broadcast/Fork/Inform 扇出）、`call`（Call/Go/SendRaw/wrap）、
  `discovery`（watch/选点/缓存客户端）、`transfer`（SendFile/DownloadFile/Stream）。

无 awkward 的跨文件依赖，无"拆错位置"的接缝（除已修的 #3）。

## 4. 验证

```
go build ./...              # 通过
go vet ./server ./client ./share   # 干净（原 t.Fatalf 报警已消除）
go test ./server -count=1          # ok (11.7s)
```

## 5. 结论

拆分本身干净，无需改动。新增逻辑修掉了 5 处质量问题（死代码、不一致守卫、接缝
垃圾、废弃 API、测试 vet 报警），核心收益是**空白名单规则从三处收敛到边界一处**、
**同族注册方法的 nil-Plugins 守卫对齐**。两项更深的设计改进（插件错误策略下沉、
白名单泛化 `suitableMethods`）超出 `/simplify` 范围，记录留待后续。
