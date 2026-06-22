Title: 给注册加一条方法白名单——只暴露你点名的方法，其余导出方法一律不开 RPC

Author(s): chaoyuepan

Last updated: 2026-06-22

Discussion at: https://github.com/smallnest/rpcx/issues/581 · tasks/prd-method-level-registration.md

Status: Draft

## Abstract / 摘要

我们给 rpcx 服务端加一组白名单式注册入口 `RegisterWithMethods` / `RegisterNameWithMethods`：调用方点名要暴露哪些方法，只有名单里的方法被注册成 RPC，其余签名合适的导出方法一概不开。白名单里写了不存在或签名不符的方法名就报错，空名单也报错——拼错名字不会静默少一个接口。

最重要的承诺：**现有 `Register`/`RegisterName` 的签名与行为逐字节不变。** 我们靠一条内部代码路径同时支撑新旧入口——给内部 `register` 加一个 `methods` 白名单参数，旧入口传 `nil` 走原全量逻辑，新入口传名单走过滤逻辑。零破坏是硬约束。

## Background / 背景与动机

rpcx 注册一个 struct，就把它**所有**签名合适的导出方法全开成 RPC。这行代码是源头：

```go
// server/service.go:172 —— register 内部
service.method = suitableMethods(service.typ, true)  // 全部合适方法，无从挑选
```

`suitableMethods` 扫遍 `typ` 的每个方法，导出且签名匹配的全部收进 `service.method`。注册完，这些方法就都能被远程调用。

问题是：**导出 ≠ 想开成 RPC。** 一个 service struct 上常有些导出方法是给同进程其他代码复用的——比如同一份业务逻辑，既要被 rpcx 暴露，又要被 HTTP handler 或 jsonrpc 调用。今天你没法说"这个方法给本地用、别开成 RPC"：要么把它改成非导出（同包代码也调不到了），要么把 struct 拆开。Issue #581 的诉求正是"注册时指定具体注册哪些方法"，作者确认为"避免暴露不该暴露的方法"，并归入 `8.0`。

一句话定性：**注册的粒度今天是"整个 struct"，而真实需求是"struct 上的一个子集"。** 缺的就是让调用方圈定子集的入口。

## Design / 设计

### 一句话：白名单是 register 的一个可选参数，旧入口传 nil

我们不另起炉灶。现有 `register(rcvr, name, useName)` 已经是 `Register` 和 `RegisterName` 共用的核心，我们给它加第四个参数 `methods []string`：`nil` 表示"全要"（旧行为），非 nil 表示"只要名单里的"。一条路径，两种入口。

#### 公开入口

```go
// 旧入口：行为、签名都不动，内部传 nil
func (s *Server) Register(rcvr interface{}, metadata string) error {
	sname, err := s.register(rcvr, "", false, nil)   // ← 多一个 nil
	...
}

// 新入口：白名单式
func (s *Server) RegisterWithMethods(rcvr interface{}, methods []string, metadata string) error
func (s *Server) RegisterNameWithMethods(name string, rcvr interface{}, methods []string, metadata string) error
```

用法（3 个导出方法只开 2 个）：

```go
type Calc struct{}
func (c *Calc) Add(ctx context.Context, args *Args, reply *Reply) error { ... } // 想开
func (c *Calc) Sub(ctx context.Context, args *Args, reply *Reply) error { ... } // 想开
func (c *Calc) internalReset(ctx context.Context, ...) error            { ... } // 仅本地

s.RegisterWithMethods(new(Calc), []string{"Add", "Sub"}, "")  // Reset 不暴露
```

#### 过滤 + 校验：在 suitableMethods 之后做一次分流

核心逻辑只有几行。`suitableMethods` 先照常算出"所有合适方法"的 map，然后按白名单过滤；对名单里每个**没命中**的名字，再分流出错误原因：

```go
func (s *Server) register(rcvr interface{}, name string, useName bool, methods []string) (string, error) {
	...
	all := suitableMethods(service.typ, true)   // 既有逻辑，全量

	if methods == nil {                          // 旧路径：全要，行为不变
		service.method = all
	} else {
		if len(methods) == 0 {                   // 空名单：报错（见 Rationale）
			return sname, errors.New("rpcx.Register: empty methods whitelist; use Register to register all methods")
		}
		picked := make(map[string]*methodType)
		for _, m := range methods {
			if mt, ok := all[m]; ok {            // 命中：合适且导出
				picked[m] = mt
				continue
			}
			// 没命中：区分"根本不存在/非导出" vs "存在但签名不符"
			if _, exists := service.typ.MethodByName(m); exists {
				return sname, fmt.Errorf("rpcx.Register: method %q of %s is not a suitable RPC method", m, sname)
			}
			return sname, fmt.Errorf("rpcx.Register: method %q not found on %s", m, sname)
		}
		service.method = picked
	}
	...
}
```

边界：过滤只决定"哪些方法进 `service.method`"，不碰方法本身的调用、编解码、selector——下游一律照旧。校验失败时 `return` 在写入 `s.serviceMap` 之前，所以**不会留下半注册的 service**。

#### 改造前 vs 改造后

```
改造前：Register(new(Calc))
  → suitableMethods → {Add, Sub, internalReset 里所有合适的}  全部开成 RPC

改造后：RegisterWithMethods(new(Calc), []string{"Add","Sub"})
  → suitableMethods 算全量 → 按 {Add,Sub} 过滤 → 只开 Add、Sub
     （名单写错名字 / 写了签名不符的方法 / 空名单 → 直接报错，不注册）
```

## Rationale / 理由与取舍

### 为什么加参数走一条路径，而不是另写一个 registerWithMethods

最朴素的做法是新写一个 `registerWithMethods` 函数，和 `register` 并存。我们没选，因为两者除了"过滤那几行"几乎完全一样——并存等于把 service 构造、pointer-receiver 提示、错误处理、写 map 这一长串逻辑抄两份，日后改一处要记得改两处。给 `register` 加一个 `methods` 参数、旧入口传 `nil`，让新旧共用同一条路径，是改动最小、最不容易长歪的接法。代价是 `register` 的签名多了一个参数，但它是非导出函数，只在包内几个调用点改一下 `nil`，不影响任何用户。

### 为什么用白名单，而不是黑名单

可以做成"排除某些方法"的黑名单。我们选白名单，因为这个特性的初衷是**安全**——"避免暴露不该暴露的方法"。白名单默认不暴露，你新加一个方法，除非显式列进名单，否则它不会悄悄变成 RPC 端点；黑名单则相反，新方法默认暴露，忘了加进黑名单就漏了。安全的默认应该是"默认关"，所以白名单。

### 为什么名单里有不存在/签名不符的方法名要报错，而不是静默忽略

静默忽略看着"宽容"，实则危险。你把 `"Add"` 拼成 `"add"`，或把一个签名不符 RPC 形态的方法写进名单，静默忽略的结果是这个接口**没被注册、却没人告诉你**——直到线上调用方收到"方法不存在"才发现。我们选报错，并且用 `MethodByName` 把"根本不存在"和"存在但签名不符"分成两条错误信息，让你一眼看出是拼错了名字还是方法签名不对。宁可注册时吵一句，也不让接口静默缺失。

### 为什么空名单报错，而不是当成"全部"或"全不要"

空名单（nil 或 `len==0`）有三种可能的语义：全要、全不要、报错。"全要"会和 `Register` 重复且容易误用（本想填名单却传了空，结果全暴露，正好踩中要避免的事）；"全不要"注册一个零方法的 service 毫无意义。我们让它报错并提示"要全部就用 `Register`"——把模糊地带关掉，逼调用方表达清楚意图。注意 `nil` 和空切片语义不同：内部 `register` 收到 `nil` 才是旧的"全量"路径（只有公开的 `Register` 这么传），公开的 `RegisterWithMethods` 收到 nil/空都按空名单报错。

## Compatibility / 兼容性

**这是纯增量变更，对现有用户零破坏。** `Register`/`RegisterName` 的公开签名不变，行为不变——它们内部给 `register` 传 `nil`，走的还是 `service.method = all` 那条老路，注册结果逐字节一致。新增的只有两个新方法和一个非导出函数的参数。

唯一的代价诚实列出：**内部 `register` 的签名多了一个参数**。这是包内改动，需要同步更新包内所有调用点（目前是 `Register`、`RegisterName` 两处，各加一个 `nil`）。对包外用户完全不可见。

没有迁移路径要写——老代码不用动，想用新能力的人改调一个新方法即可。

## Implementation / 实现与过渡

改动集中在一个文件 `server/service.go`，分三步、可独立验证：

1. **改内部 `register` 签名**加 `methods []string`，实现"nil 走全量、非 nil 走过滤+校验"的分流；更新 `Register`/`RegisterName` 两处调用传 `nil`。
2. **加两个公开入口** `RegisterWithMethods`/`RegisterNameWithMethods`，各自把名单透传给 `register`，注册成功后照旧走 `Plugins.DoRegister`。
3. **补单测**：白名单子集生效、名单含不存在方法报错、含签名不符方法报错（两条错误信息不同）、空名单报错、且任一错误下 service 未进 `serviceMap`。

可落地性靠证据：

- **行为不变的证据**：现有 server 测试全绿，证明 `Register`/`RegisterName` 没回归。
- **新能力的证据**：新单测覆盖"只暴露子集"和四类错误分支；`go test ./server/` 通过。
- **形状的证据**：白名单过滤后 `service.method` 的 key 集合等于名单集合，可直接断言。

## Open Questions

- 方法名匹配是否需要大小写不敏感？（当前定：精确匹配——Go 方法名本就大小写敏感，跟随语言语义。）
- 是否提供一个辅助函数列出"某 struct 上所有可注册方法名"，方便用户构造白名单？（倾向后续增强，不进本版。）
- `RegisterFunction` 系列要不要类似能力？（当前定：不在范围——函数级注册本就是单个，没有子集问题。）
