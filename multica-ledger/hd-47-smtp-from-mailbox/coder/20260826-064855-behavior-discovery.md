---
ts: 20260826-064855
agent: coder
action: BEHAVIOR_DISCOVERY
tree: —
verdict: FROZEN
---

# HD-49 Stage 1：SMTP_FROM_EMAIL 行为面 inventory

本 Run 只做发现与枚举。**未改动任何 `server/**` 生产代码或测试代码**，未提修复方案，
未预判 `mail.ParseAddress` 是否正确。本次 diff 只有 `multica-ledger/**`。

front matter 的 `tree` 记为 `—`：本分录不改动任何业务代码，被观测的对象是 base tree
`9fb8974cdc4038f9c42a99f3fbf1613729f89def`，inventory 的 `subject` 块已绑定它。

## 编排事实（读回确认，非转述）

| 项 | 值 | 确认方式 |
|---|---|---|
| Repo | `https://github.com/xiaoyaner0201/multica` | `git remote -v` |
| 分支 | `hd-47/smtp-from-mailbox`（沿用，未新建） | `git checkout -B ... origin/hd-47/smtp-from-mailbox` |
| BASE | `620f99d8ce168ee56bfcfacf359b3bcca30f21bf` | `git log --oneline -5` |
| base tree | `9fb8974cdc4038f9c42a99f3fbf1613729f89def` | `git rev-parse 620f99d8c^{tree}`，与 index.md 头部一致 |
| 起点 HEAD | `be648eead6b7554c30312374beea887e2ca543b9`（leader intake 分录） | `git rev-parse HEAD`，工作树 clean |
| Go 工具链 | go1.27.0 linux/arm64 | `go version` |

父 Issue 正文里的 `8fda4f22d` 已过期，未使用。

## 产出

| 文件 | sha256 |
|---|---|
| `coder/20260826-064855-behavior-inventory.json` | `b4d374f29489178f4fa90faa9df74abfb40db85464963b7c8c13d16a29670491` |
| `coder/20260826-064855-sink-probe.go.txt` | 见下方 sha 表 |
| `coder/20260826-064855-sink-probe-output.txt` | 见下方 sha 表 |
| `coder/20260826-064855-inventory-schema-check.txt` | 见下方 sha 表 |
| `coder/20260826-064855-validate-inventory-only.py.txt` | 见下方 sha 表 |

sha256 全表见本目录 `20260826-064855-artifact-sha256.txt`。

- schema：`behavior-inventory.v1`
- candidate 数量：**43**（C01–C43）
- `maturity`：**D1**（`must_fail_old` 为 true 的 9 条：C03、C11、C12、C25、C26、C27、C28、C29、C30）
- `produced_by_run_id`：`8c198cbd-edf5-4a57-8936-4c42456c9b50`
- `produced_by_context_id`：`agent:fd2d1252-2974-4998-8b7e-128c7c7c2c36/container-slot:4c42456c9b50/issue:8d1cabf6-3113-40bd-9e01-f968644c7819`

两者均由本 Coder Run 产生，与后续 Plan Run/context 天然不同；validator 的
provenance 检查在 Plan Gate 时应当成立。

## 为什么是 D1，不是 D2

本仓库 Go 侧没有 AST / Semgrep / CodeQL / 事件注册表级别的确定性发现器。
`multica-gate/fixtures/` 下只有 `skillhub-java` 的历史夹具，属该仓库专属，**未调用、未复制**。
因此本次是一次结构化人工枚举，`generator.mode` 如实写明方法组合，`coverage_note`
写清依据与盲区。按协议 D1 会在 receipt 上带强制 warning，这是预期结果。

支撑枚举可信度的三点，也一并写在 `coverage_note` 里：

1. 整仓 ripgrep 在这里是**可穷尽的**——整个 Go module 只有一个 `net/smtp` 导入方和一个
   `resend-go` 导入方，都是 `server/internal/service/email.go`。
2. `email.go`(434 行) 与 `email_test.go`(730 行) 全文逐行读过。
3. 关键事实不靠推理，靠抓包：见下节。

## Sink 层实测（不是推断）

在仓库之外写了一个一次性探针，**不 import 本仓库**，只按当前 tree 的发射顺序
（`c.Mail(raw)` → `"From: " + raw`）对一个宽容 SMTP 捕获服务器重放 10 种取值形状，
记录收到的字节。宽容是刻意的——它正是父 Issue 警告 QA 的那个盲区。

关键结果（完整输出见 `20260826-064855-sink-probe-output.txt`）：

```
input "sender@example.com"
  C: MAIL FROM:<sender@example.com> BODY=8BITMIME
  D: From: sender@example.com

input "\"Example Team\" <sender@example.com>"
  C: MAIL FROM:<"Example Team" <sender@example.com>> BODY=8BITMIME
  D: From: "Example Team" <sender@example.com>

input "示例团队 <sender@example.com>"
  C: MAIL FROM:<示例团队 <sender@example.com>> BODY=8BITMIME
  D: From: 示例团队 <sender@example.com>          ← 裸 UTF-8，无 RFC 2047

input "<sender@example.com>"
  C: MAIL FROM:<<sender@example.com>> BODY=8BITMIME   ← 双层尖括号

input "sender@example.com\r\nBcc: evil@example.com"
  ERR: MAIL FROM rejected at client: smtp: A line must not contain CR or LF
```

`net/smtp` 侧的两条事实也直接读了 stdlib：`Client.Mail` 的格式串是
`"MAIL FROM:<%s>"`（go1.27.0 `net/smtp/smtp.go:252`），`validateLine` 拒绝 CR/LF
（同文件 425–430）。

## 三点值得 Planner 单独看的发现

### 1. CR/LF 防护是**位置性**的，不是内在的（C13 / C31）

`email.go:319` 的 `"From: " + s.fromEmail` 自己**没有任何 CR/LF 校验**。今天注入进不去，
唯一原因是 `c.Mail`（L309）先跑、`validateLine` 先 abort。也就是说：header 注入面现在
不可达，靠的是调用顺序。任何改变「交给 `c.Mail` 的是什么」的方案，都可能让 display-name
部分不再经过那道 stdlib 检查，从而把这个面从不可达变成可达。

这里我把 `must_fail_old` 如实标成 **false** 并写明原因：今天一个断言「没有 Bcc 被注入到
header」的探针是**会通过**的，它不是合法的 RED 探针，QA 不能拿它当 RED。父 Issue 完全
没提这个面；dispatch 要求它作为独立 candidate，确实必要。配置是 operator 控制的，
不是终端用户控制的，所以严重性有界——但这是"不该被静默回归"的一条。

### 2. Resend 是本次改动**新可达**的 consumer（C20 / C21 / C22）

两条路径读的是**同一个字段**，由**同一个 resolver**（C06，全仓唯一生产写点）写入。
父 Issue 把 Resend 列为 non-goal，但 non-goal ≠ 不受影响：任何"存解析后的值而不是原始串"
的方案，会自动改变 Resend 的 `From`。因此 C20/C21 我**刻意挂上了 `INV6-resend-unaffected`**
——按 validator 的规则（`validate_gate.py:150-157`），带 `affected_invariants` 的 candidate
**不能**被 Planner 以 `non-goal` 排除，必须 planned 并给证据。

顺带排掉一个假阳性：C22。`email.go:376` 在 **SMTP 分支里**也调了
`buildInvitationParams(s.fromEmail, ...)`，但返回的 `.From` 被丢弃（只用 Subject/Html）。
只看 grep 输出很容易把它误建模成第三个发射点。

### 3. 现有测试**根本观测不到**这两个 sink（C39）

`email_test.go:610-611` 的 mock server 对 `MAIL FROM:` 一律回 `250 OK`，**不保存那一行**；
`616-625` 的 DATA 分支读到 `.` 为止，**逐行丢弃**。三个测试
（`TestSendSMTP_FallbackReconnectsAndAuthsWithLOGIN` / `_PlainAuthSucceedsWithoutFallback` /
`_NoAuthWhenUsernameEmpty`）都只断言 `err == nil`。

结论直说：**这三个测试在信封完全畸形的情况下也会通过**。任何针对 C11/C12/C25–C27 的
RED 探针，必须**先**把这个 harness 改成保留收到的行，否则断言的只是"发送成功"，
正是父 Issue 叮嘱 QA 要避免的假阴性。

## 其余枚举要点（逐条对齐 dispatch 的 7 项范围）

1. **`s.fromEmail` 全部读写点 + `resolveFromEmail` 每条返回路径** → C01–C07。
   - 生产写点唯一：`NewEmailService` L187→L242（C06）。测试直写 4 处（C07），**绕过 resolver**
     ——放在 resolver/构造器里的校验不会被这 4 个测试覆盖。
   - 返回路径 4 条全覆盖，含 `RESEND_FROM_EMAIL` 回退（C04）与 `noreply@multica.ai` 默认（C02）。
   - **不对称事实**：默认值只存在于 Resend 分支；SMTP 分支没有默认，可以合法返回 `""`。
   - **易漏的语义**（C05）：whitespace-only 在 L122 被 TrimSpace 成 `""`，于是**回退到
     `RESEND_FROM_EMAIL`**，并不会走到 L258 的报错——只有两个都空才报错。修复时若把
     whitespace-only 重新解读成"格式非法"，实盘上会换掉发件地址。
2. **`sendSMTP` 的每一个上游 caller，回溯到公开入口** → C17、C18，**没有第三条**（C19，
   作为"已枚举的否定"显式记账）。
   - C17 `POST /api/auth/send-code`（router.go:1384），**未认证**公开端点，同步 500。
   - C18 `POST /api/workspaces/{id}/members`（router.go:1609），owner/admin 才可，且在
     **detached goroutine 里 fire-and-forget**（invitation.go:239-243）——响应已经写完，
     失败只进 log。QA 在这条路径上断言 HTTP 响应等于什么都没证明。
3. **Resend 如何消费同一个 fromEmail** → 见上文第 2 点。
4. **`SMTP_USERNAME` 与发件身份的耦合** → C23：**当前 tree 上不存在耦合**。
   `resolveFromEmail` 全函数体（L114-126）只读 `RESEND_FROM_EMAIL` 与 `SMTP_FROM_EMAIL`；
   git 追溯到 `5b57a8ebc`（MUL-4460 / 上游 PR #5322）确认边界是那次刻意建立的。
   **但覆盖有缺口**：`TestNewEmailService_FromEmailResolution` 在 fixture 里设了
   `SMTP_USERNAME`（L241）却从不断言否定面——不存在"两个 from 变量都空 + 设了不同的
   `SMTP_USERNAME`"的用例，而那**正是**意外引入 fallback 时唯一会暴露的状态，
   现有测试全绿也发现不了。另注：`SMTP_USERNAME` 是这几个变量里唯一没包 TrimSpace 的。
5. **最终 sink** → C11（信封字节）、C12（`From:` 头字节）、C14（stdlib 追加的
   `BODY=8BITMIME` / `SMTPUTF8` 信封参数）、C15（同批头部）、C16（失败路径是否带出发件配置）。
   - C15 记下来是为了**防止探针过度断言**：`Subject` 走了 `mime.QEncoding.Encode`（L292）
     而 `From` 没有——这个不对称就在同一个字符串字面量里；`Date`/`Message-ID` 用
     `time.Now()`，非确定性，golden-bytes 断言必须排除；`Message-ID` 的域名取的是
     `s.smtpHost` 而不是发件域，属既有行为，不在本次范围。
   - C16：L259 的配置错误**不含取值**，HTTP 响应体是固定串（auth.go:359），
     **对客户端无泄漏**——这条是源码确认的。未确认的部分见 UNVERIFIED。
6. **失败/陈旧状态** → C24（bare，回归锚点）、C25/C26（ASCII display name 引号/非引号）、
   C27（CJK）、C28（仅 angle-addr）、C29（不可解析）、C30（多 mailbox）、C31（CR/LF）、
   C32（空/纯空白）。每条都有实测字节。
   - C28 顺带是一个**双向兼容陷阱**：今天配 `<a@b>` 的 operator 已经是坏的，
     所以"保持今天的行为"和"接受合法 mailbox"在这里指向相反方向。这是 Planner 的决定，不是我的。
   - C30 在 RFC 层面本身就没有唯一正解：RFC 5322 允许多 mailbox 的 From，SMTP 信封只允许
     一个 reverse-path。今天是静默发一个畸形信封。
7. **现有测试覆盖了哪些、哪些没有** → C37–C40，逐条写明 COVERS / DOES NOT COVER。
   baseline 已实跑：`go test ./internal/service/ -run 'Email|SMTP|Invitation|Login|Sanitize' -v`
   全部 PASS（`TestFailTaskSanitizesNULDiagnostic` SKIP，原因是本容器无 DB，与本 Issue 无关）。

## `must_fail_old` 的判断口径

按 dispatch 要求，逐条判断"能否在**当前** tree 上被一个探针证伪"，不放水：

- **true（9 条）**：C03、C11、C12、C25、C26、C27、C28、C29、C30。都有实测字节支撑，
  QA 的 RED 敏感性可以直接打在这些上面。
- **false 且需要说明的**：C31（见上文第 1 点，今天探针会 PASS，不是合法 RED）、
  C24（回归锚点，今天就该 PASS）、C32（今天行为正确，被 INV2 钉住）、
  C37–C40（描述的是**测试覆盖能力**，不是生产行为，本身不是 RED 目标）。

## UNVERIFIED（6 条，均已作为 candidate 落账，不默认安全）

| id | 未确认的是什么 | 为什么确认不了 |
|---|---|---|
| C14 | `SMTPUTF8` 信封参数分支 | 探针服务器与现有测试都不 advertise SMTPUTF8，无 live relay。`BODY=8BITMIME` 分支已实测确认 |
| C16 | 真实 relay 是否把出错的 MAIL FROM 参数回显进 5xx 文本、经 `%w` 落进 server log | 无 live relay。父 Issue 的阿里云抓包是 `500 Error: bad syntax`（无回显），单一 relay 不可外推 |
| C35 | 8 个非 `.env.example` 文档面（4 语种 × environment-variables/auth-setup）是否在本次范围 | 父 Issue 的 In-scope 只点名 `.env.example`，属 Source Contract 问题。仓库先例是四语种同 commit 改（`5b57a8ebc`） |
| C41 | 仓库外的 env 注入通道（k8s secret / systemd / PaaS / CI） | 从 tree 里枚举不到。影响"向后兼容"能被验证到什么程度：只能验形状（C24–C32），验不了实盘取值分布。与 startup-time vs send-time 校验的选择直接相关 |
| C42 | 仓库外是否有日志管道消费 `from=` 启动行 | 从 tree 里枚举不到。仓库内无消费方 |
| C43 | Go stdlib 版本漂移 | 本 Run 读的是 go1.27.0；父 Issue 引的是 go1.25.6。两者在 `"MAIL FROM:<%s>"` 与 `validateLine` 上一致，但 C14 的参数分支未跨版本核对，CI 用哪条工具链也未确认 |

C35/C41/C43 我判断是**会影响 Plan 决策**的，不只是形式上的留白。

## Gate 校验

`run_gate.sh` 没有 `--kind inventory` 模式（`choices=["plan","qa","review"]`，
`scripts/validate_gate.py:503`），且三种模式都要求一个 Stage 1 尚不存在的 Plan/QA/Review
artifact，所以无法在本 Stage 跑完整 gate。改为**复刻 `validate_gate.py:514-522` 对 inventory
做的那一段检查**：同一份 schema 文件、同一个 `jsonschema==4.25.1`。脚本与输出都已随分录提交。

Bundle 按纪律从本容器 skill 目录复制到 `/workspaces/HD-49/gates/` 后执行，未从 `$HOME` 或
`/tmp` 运行。

```
schema: PASS  (behavior-inventory.v1)
schema_file_sha256   6948ac7e9da372dbd209014975dad878bb33e0399a0136f268ad90a3306c2a79
inventory_sha256     b4d374f29489178f4fa90faa9df74abfb40db85464963b7c8c13d16a29670491
jsonschema_version   4.25.1
candidates           43 unique
maturity             D1
produced_by_run_id   8c198cbd-edf5-4a57-8936-4c42456c9b50
produced_by_ctx_id   agent:fd2d1252-.../container-slot:4c42456c9b50/issue:8d1cabf6-...
WARNING (mandatory): D1 discovery — NOT an upper bound on the behavior surface.
```

完整 Plan Gate（provenance、闭包、digest 链）仍需在 Planner 产出 plan artifact 后由调度跑
`run_gate.sh --kind plan --artifact <plan> --inventory <本文件>`。

## 边界自查

- ✅ 未改 `server/**` 生产代码或测试代码；本 Stage diff 只有 `multica-ledger/**`
- ✅ 未提修复方案，未预判 `mail.ParseAddress`
- ✅ 未新建分支，未 push 公开分支，未建上游 PR
- ✅ 未跨仓库调用 `fixtures/skillhub-java`
- ✅ 未标 `D0`
- ✅ 账本内无凭据
- 探针源码在 `/tmp`，构建产物在 `/workspaces/.../.probe-build`（均在仓库外，未进 tree）；
  源码与输出已归档进本目录以便复现

无 `PLAN_GAP`：本 Stage 尚无 Plan 可比对。无 `RISK_ROUTE_GAP`：dispatch 已标 `HIGH_RISK`，
与我在枚举中命中的风险面（认证材料投递 sink、notification fan-out、现网全量邮件停发、
异步 goroutine 投递、header 注入面）一致，路由正确。

## 交接

- **状态**：Discovery 完成，inventory **冻结**
- **产物**：分支 `hd-47/smtp-from-mailbox`
  - inventory：`multica-ledger/hd-47-smtp-from-mailbox/coder/20260826-064855-behavior-inventory.json`
  - sha256：`b4d374f29489178f4fa90faa9df74abfb40db85464963b7c8c13d16a29670491`
  - schema：`behavior-inventory.v1`；maturity **D1**；candidate **43**；`must_fail_old=true` **9** 条
  - UNVERIFIED **6** 条：C14、C16、C35、C41、C42、C43
- **下一步**：调度跑 Plan Gate 前置校验并派 Planner。Planner 消费本 inventory，
  **不得修改**；需对 C01–C43 **恰好一次**做 `planned` / `excluded` 处置，`unresolved` 必须为空。
- **给 Planner 的硬提醒**：
  1. 带 `affected_invariants` 的 candidate（含 C20/C21 两条 Resend）按 validator 规则
     **不能**以 `non-goal` 排除；
  2. C31 的 `must_fail_old=false` 是如实判断，不要当成"这里不需要探针"；
  3. C39 说明现有 SMTP harness 观测不到目标 sink——任何 RED 计划要先把这件事算进工作量；
  4. C41/C43 直接影响 startup-time vs send-time 校验的取舍，父 Issue 把这个选择留给了 Planner。
- **给 Reviewer 的强制提醒**：D1 inventory **不是行为面的完整性上界**，
  `BLIND_BEHAVIOR_AUDIT` 必须独立追踪，不得以"inventory 没列"推出"系统里没有"。
- **未解决**：上表 6 条 UNVERIFIED
- **需要决策**：C35（文档范围）属 Source Contract 问题，Planner 需明确处置
