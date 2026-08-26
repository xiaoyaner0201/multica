---
ts: 20260826-070548
agent: coder
action: BEHAVIOR_DISCOVERY — behavior-inventory.v1
tree: —
verdict: FROZEN
---

## 本 Run 的边界

只做 discovery。**未写 Plan、未写实现代码、未提方案、未修改 `server/` 任何文件。**
本分录及其产物只描述 base tree `bfb2d1ef8bf8861cdadc3bdd6e1ce35f6bf48cee` 上的既有行为，
以及该行为面上「改动会碰到什么」的枚举。任何取舍留给 Planner。

`tree` 记为 `—`：本 Run 只新增 `multica-ledger/**`，业务路径 diff 为空。

## 产物

| 文件 | 内容 |
|---|---|
| `coder/behavior-inventory.v1.json` | 冻结的 `behavior-inventory.v1`，24 条 candidate |
| `coder/discovery-ast-raw.json` | 发现器原始输出（确定性、可复现） |
| `coder/tools/discover_email_behavior.go` | 发现器源码（`//` 说明其自身局限） |
| `coder/tools/go.mod` | 独立嵌套 module，不属于 `server/` module，不影响 `go build ./...` |

inventory sha256：`fbc4604be3ddaf6d254a98ff3d5fb33e45fee56bd06cd5b4af8351fccdcbc8ee`

Schema 校验（Gate bundle 已按纪律复制到 `/workspaces/HD-48/gates/`，未从 `$HOME` 或 `/tmp` 运行）：

```
cd /workspaces/HD-48/gates && UV_CACHE_DIR=/workspaces/.skill-cache/multica-gates-uv \
  uv run --with 'jsonschema==4.25.1' python -c "...Draft202012Validator(behavior-inventory.schema.json)..."
behavior-inventory.v1 SCHEMA VALID
candidates = 24
maturity   = D1
```

> 注：`UV_CACHE_DIR` 必须显式指向 `/workspaces` 下。`run_gate.sh` 里那段 fallback 只在
> `/workspaces/.skill-cache` 已存在且可写时才生效；否则 uv 会把带原生扩展的 wheel 解到
> `$HOME/.cache`，而 Runtime HOME 是 `noexec`，`rpds` 会以
> `failed to map segment from shared object` 失败。本 Run 实测踩到过，已记于此供后续 Gate 复用。

## generator provenance

| 字段 | 值 |
|---|---|
| `produced_by_run_id` | `02cc6075-02d4-448e-8225-25158762e98b` |
| `produced_by_context_id` | `25158762e98b` |
| `maturity` | `D1` |

两个标识都是本 Run 的真实 Multica 标识：run id 即本容器的 `MULTICA_TASK_ID`，
context id 即本 Run 的 task-context 目录 `/workspaces/<workspace-id>/25158762e98b/`。
无占位符、无编造。Plan Run 是另一个 task，两者机械不等。

## 为什么标 D1 而不是 D2 —— 与 dispatch 的一处口径分歧

dispatch 写的是「能给出确定性可复现命令就标 `D2`」。**我给得出确定性可复现命令，但仍然标 `D1`。**

理由是 `multica-engineering-delivery` 正本对分级的定义不是按「命令是否可复现」，而是按解析能力：

> D2 通用静态分析：AST / Semgrep / CodeQL，**跨文件解析调用与注解**

我写的发现器确实是 `go/ast` 解析（不是正则数花括号），确实跨全部 1555 个非 vendor `.go` 文件，
确实输出确定性排序结果。但它**按 selector 名字匹配调用目标，没有 `go/types` 类型解析**——
本环境 module cache 里没有 `golang.org/x/tools`，离线拿不到 `go/packages`。
名字匹配不等于「解析调用」，所以按正本口径它够不到 D2。

按角色约束「发现 skill 与 dispatch 矛盾时按 skill 执行并报告」，此处按 skill 执行并在此报告。
实际后果是 Plan Gate 会对 D1 打一条强制 warning：**inventory 不是行为面的完整性上界，
Reviewer 的 `BLIND_BEHAVIOR_AUDIT` 必须独立追踪**。对一个改写登录凭据邮件的 HIGH_RISK 任务，
这条 warning 是正确信号，不是需要绕开的噪声。

要真正升到 D2，需要在该仓库补 `go/packages` 或 Semgrep 规则——那是独立投入，不在本 Issue 写集内。

## 发现器做了什么

```
cd multica-ledger/feat-hd-48-email-template-override/coder/tools
go run . -root ../../../../server -prefix ../../../.. > ../discovery-ast-raw.json
```

四个维度，全部按 `file` 再按 `line` 排序，输出确定性：

1. **调用点**（全仓）：`SendVerificationCode` / `SendInvitationEmail` / `sendSMTP` /
   `buildInvitationParams` / `sanitizeSubjectField` / `NewEmailService` / `resolveFromEmail` /
   `openSMTPClient`，带 enclosing 函数、是否在 `go` 语句内、是否在 `defer` 内。
2. **sink**（限 `email.go`）：`Emails.Send` / `fmt.Printf` / `fmt.Println` / `fmt.Fprintf` /
   `c.Mail` / `c.Rcpt` / `c.Data`。
3. **转义与编码点**（限 `email.go`）：`html.EscapeString` / `sanitizeSubjectField` /
   `mime.QEncoding.Encode` / `quotedprintable.NewWriter`。
4. **收件人可见字面量与 env key**（限 `email.go`）。

`go test` 基线（base tree，绿）：

```
GOTMPDIR=/workspaces/HD-48/gotmp go test ./internal/service/ \
  -run 'TestSanitizeSubjectField|TestBuildInvitationParams|TestSendSMTP|TestNewEmailService|TestLoginAuth|TestSMTPAuth'
ok  github.com/multica-ai/multica/server/internal/service  0.026s
```

> 顺带一条环境事实：默认 `TMPDIR` 是 `noexec`，`go test` 会以
> `fork/exec .../service.test: permission denied` 失败。必须 `GOTMPDIR` 指到 `/workspaces` 下。
> QA 会再跑一次同样的命令，这条先记下省得重踩。

## 24 条 candidate 的结构

| 组 | candidate | 说明 |
|---|---|---|
| 验证码三 sink | `VC-SINK-SMTP` / `VC-SINK-RESEND` / `VC-SINK-DEV` | subject 在 SMTP 与 Resend 是两份独立字面量；DEV 分支根本不渲染 subject/body |
| 邀请三 sink | `INV-SINK-SMTP` / `INV-SINK-RESEND` / `INV-SINK-DEV` | subject 单源于 `buildInvitationParams`；DEV 分支**两套转义模型全绕过** |
| SMTP header | `SMTP-HEADER-SUBJECT-QENCODE` / `-CTE-8BIT` / `-CTE-QP` / `-CONTENT-TYPE` | CJK subject 验收项落点；8bit 与 QP 是同一模板的两套 wire bytes |
| 转义模型 | `ESCAPE-MODEL-BODY-HTMLESCAPESTRING` / `ESCAPE-MODEL-SUBJECT-SANITIZE` | 两条语义不同，分别登记 |
| 入口与来源 | `ENTRY-SENDCODE-HANDLER` / `ENTRY-CREATEINVITATION-HANDLER` / `INPUT-INVITERNAME-PROVENANCE` / `INPUT-WORKSPACENAME-PROVENANCE` | 真实 HTTP 入口 + 两个变量的上游可控性 |
| 生命周期 | `CTOR-NEWEMAILSERVICE-STARTUP` / `OVERRIDE-NEW-INPUT-SURFACE` | 启动期是唯一能「响亮失败」的位置 |
| 既有测试 | `TEST-EXISTING-INVITATION-SANITIZATION` / `TEST-GAP-WIRE-BYTES` | 登记为 evidence，RED 敏感性交 QA 判 |
| 部署面 | `DEPLOY-ENV-PLUMBING` | compose / helm / .env.example / 四语种文档 |
| UNVERIFIED | `UNVERIFIED-DYNAMIC-DISPATCH` / `UNVERIFIED-RESEND-SDK-BEHAVIOR` / `UNVERIFIED-NONGO-SURFACES` | 不默认安全 |

## 五处值得 Planner 先看的事实

这五条不是方案，是**读源码读出来、且不在 Issue 行号表里**的事实。

### 1. 邀请邮件是 detached goroutine，「响亮失败」在发送期结构上不可能

`handler/invitation.go` 里是 `go func() { ...SendInvitationEmail... }()`，
错误只有 `slog.Warn`，HTTP 201 早已写出。发现器把这个调用点标了 `in_go_stmt: true`。

Issue 不变量 4 要求「模板损坏必须响亮且尽早失败，不能中途静默回退」。在这条路径上，
**发送期没有任何可以响亮的通道**。唯一能满足该不变量的位置是进程启动期。

### 2. 但 `NewEmailService()` 不返回 error

签名是 `func NewEmailService() *EmailService`。今天没有任何通道能让模板加载失败中断启动。
要满足不变量 4，必须改 `cmd/server/router.go:421` 这个唯一构造点的签名或生命周期——
**那是 Issue 行号表之外的文件**。而且该函数现有的错误处理先例全是 log-and-continue
（`os.Hostname()` 失败、`SMTP_TLS` 值不认识，都只 `Printf` 一句），
而 log-and-continue 恰好就是不变量 4 明令禁止的静默回退。这个取舍必须由 Planner 显式做并说明理由。

### 3. DEV sink 绕过全部转义，且是第三份文案副本

```go
fmt.Printf("[DEV] Invitation email to %s: %s invited you to %s — %s\n", to, inviterName, workspaceName, inviteURL)
```

`inviterName` / `workspaceName` **原样**打印：不过 `html.EscapeString`，不过 `sanitizeSubjectField`。
控制字符、200 字符长名、markup 今天全部原样进日志。
另外 "invited you to" 这句话现在存在于三处：DEV 字面量、subject 格式串、body 字面量。
无论保留还是修正都是行为变更，都要带证据。

### 4. body 插值顺序不是按名字左到右

```go
`...<h2>You're invited to join %s</h2>
 <p><strong>%s</strong> invited you to collaborate in the <strong>%s</strong> workspace...`,
 safeWorkspace, safeInviter, safeWorkspace, inviteURL)
```

顺序是 workspace、inviter、workspace、url。模板重写时若按名字顺序排占位符，
inviter 与 workspace 会被静默对调——**而现有 5 条 substring 断言仍然全绿**，
因为两个字符串都还「出现在 body 某处」。

### 5. 现有 SMTP 测试把 wire bytes 全丢了

`testSMTPServer.handleConn` 的 DATA 分支读到单独一行 `.` 为止，**每一行都丢弃**，从不记录。
因此全树**没有任何一条断言**碰过 `Subject:` header、`mime.QEncoding` 输出、
`Content-Type`、`Content-Transfer-Encoding`、8bit 分支、quoted-printable 输出或 wire 上的 body。
且该 mock 从不 advertise `8BITMIME`，所以 8bit 分支零覆盖。
更直接的：**`SendVerificationCode` 全树零测试**——发现器在 1555 个文件里找不到它在任何
`_test.go` 中的调用点。承载登录凭据的那个 producer，正好是没有覆盖的那个。

Issue 的 CJK subject 验收项在当前 harness 下**不可观测**。

## 按 dispatch 的第 5 点

`FRONTEND_ORIGIN` 与 `s.fromEmail` **只出现在相关 candidate 的 `required_states` 里，未展开成 candidate**。
sender 地址是 HD-47 的写集，本 Issue non-goal，两个 Issue 必须独立可回滚。
`resolveFromEmail` 虽然被发现器扫到（`email.go:187`），也只作为 `CTOR-NEWEMAILSERVICE-STARTUP`
的一条 `required_states` 登记，不建模。

## 交接

- 状态：DISCOVERY 完成，inventory 已冻结
- 产物：分支 `feat/hd-48-email-template-override`；
  `multica-ledger/feat-hd-48-email-template-override/coder/behavior-inventory.v1.json`，
  sha256 `fbc4604be3ddaf6d254a98ff3d5fb33e45fee56bd06cd5b4af8351fccdcbc8ee`，24 条 candidate，
  base `1ef1d65be` / tree `bfb2d1ef8bf8861cdadc3bdd6e1ce35f6bf48cee`
- 下一步：调度校验 provenance 后派 Planner；Planner 对 24 条 candidate 各恰好一次
  `planned` 或 `excluded`，`unresolved` 必须为空。
  三条 `UNVERIFIED-*` 与 `DEPLOY-ENV-PLUMBING` 不得仅以 `non-goal` 排除——
  它们分别涉及方法局限、第三方不可观测面和「功能对目标用户是否真的生效」。
- 未解决：
  - `maturity: D1`。Plan Gate 会打强制 warning，Reviewer 不得以本 inventory 为完整性上界。
  - Issue 正文断言「镜像内不存在模板文件（`/app` 只有二进制、`migrations/`、`LICENSE`）」，
    本 Run **未复验**该断言：树内只有根 `Dockerfile` 与 `Dockerfile.web`，backend 镜像从何构建未确认。
    已登记为 `UNVERIFIED-NONGO-SURFACES`。Planner 要么复验，要么显式作为假设承担。
  - `sanitizeSubjectField` 注释断言「Resend 也会过滤 CR/LF」，该断言在本树内无法验证，
    已登记为 `UNVERIFIED-RESEND-SDK-BEHAVIOR`。不要让注释当证据用。
- 需要决策：无。本 Run 不提方案，取舍全部留给 Planner。
