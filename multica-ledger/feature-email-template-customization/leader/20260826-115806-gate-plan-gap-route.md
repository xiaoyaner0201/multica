---
ts: 20260826-115806
agent: leader
action: gate
tree: 37fb17c83b3da66a0cf25107afc9785e63e6c29a
verdict: RETURN
---

# 核对：实现上报的两处 `PLAN_GAP` 均成立，路由规划修订，冻结方案解冻

实现在 HD-53 上报 `PLAN_GAP` 并 RETURN，分录 `coder/20260826-115329-plan-gap.md`。
本条是我的独立核对与路由裁决。**结论：两处 finding 全部成立，方案 r1 解冻，退回规划出 r2。**

## 1. 事实核对（自己重算，未采信报告文字）

| 项 | 声明 | 我的复算 | 结果 |
|---|---|---|---|
| 分录 sha256 | `064d17f9…c303567` | `sha256sum` 实测同值 | 一致 |
| 账本 commit | `418d8f56a2e93979780b9ba1e4afe242a9cfd4af` | `git rev-parse HEAD` 同值 | 一致 |
| 远端读回 | 已用完整 refspec push | `git ls-remote` 同值 | 一致 |
| 工作树 clean | 是 | `git status --porcelain` 空 | 一致 |
| 无代码改动 | 无生产/测试代码 diff | `git diff --stat BASE..HEAD -- . ':(exclude)multica-ledger'` 输出为空 | 一致 |

r2 身份判据（远端 ref / commit 图）本次在实现容器一次通过，无新增摩擦。

## 2. PG1 成立：INV2/§7.1 的清洗顺序与 T08 的字节预期互斥

方案内部原文（三处，互相矛盾）：

- `:273` INV2：「自定义主题**渲染后**同样先剥控制字符再进 `QEncoding`」，其验收列只要求
  「header 块只有一行 `Subject:` 且无 `Bcc:`」——**与剥离顺序自洽**。
- §7.1 末段：「主题的防线是**渲染后**依次经过控制字符剥离、200 rune 上限、以及 SMTP 路径上的
  `QEncoding`（实测已把 CRLF 编成 `=0D=0A`）」——**前半句要求剥离，括号里的观察却预设 CRLF 还在**。
- `:490` T08 验收：「CRLF 以 `=0D=0A` 出现在编码后的主题中（INV2）」——**把那个括号升格成了验收项**。

我独立写探针复算（不复用实现的探针），原始输出：

```
PG1 rendered  = "Hi\r\nBcc: evil@example.com"
PG1 stripped  = "HiBcc: evil@example.com"
PG1 Q(stripped) = "HiBcc: evil@example.com"  <- INV2/§7.1 的顺序产出
PG1 Q(rendered) = "=?utf-8?q?Hi=0D=0ABcc:_evil@example.com?="  <- T08 预期，要求不剥离
```

**两者不可能同真。** 实现所述与实测完全一致，判断正确。

根因是那个括号：`QEncoding` 会把 CRLF 编成 `=0D=0A` 是一句关于 `QEncoding` **孤立行为**的
真观察，但在「先剥离」的管线里它永远不会被触发。方案把一个**离线观察**直接写成了
**管线末端的验收预期**，中间隔着一步会让它失效的处理却没被考虑。

### 给规划的一条实测输入（事实，不是裁决）

若规划倾向选项 2（保留 CRLF 交给 `QEncoding`），有一处需要在 r2 里显式论证：
`mime.QEncoding.Encode` **并非无条件编码**——纯 ASCII 可打印串会原样返回（上面
`Q(stripped)` 就是原样返回的证据）。CR(13)/LF(10) 低于 `0x20`，会触发编码，所以选项 2
对 CRLF **这一个** case 安全；但这条安全性依赖「`QEncoding` 必然触发」，属于隐式前提，
r2 若走选项 2 必须把它写成显式不变量并给出验收，不能默认。

**选哪一条是规划的裁决，我不代决，也不在本条里预设。**

## 3. PG2 成立：§7.1 规则 1 对 Go 模板语义的断言与事实不符

方案 §7.1 三条硬规则第 1 条：

> 「绝不传 `map[string]any`——结构体上引用不存在的字段是**解析期**错误（启动时就能告警回落）」

`:496` T14 据此要求「**解析期**即失败 → 启动告警 + 回落」。

我独立复算，`html/template` 与 `text/template` 两条路径都测（主题走 `text/template`，
正文走 `html/template`，方案 §7.1 自己规定的），原始输出：

```
PG2 html/template Parse err = <nil>
PG2 html/template Execute err = template: body:1:5: executing "body" at <.SMTPPassword>: can't evaluate field SMTPPassword in type main.verificationTemplateData
PG2 text/template Parse err = <nil>
PG2 text/template Execute err = template: subj:1:2: executing "subj" at <.SMTPPassword>: can't evaluate field SMTPPassword in type main.verificationTemplateData
```

`Parse` 不接收数据类型，只做语法分析，**两条路径均 `Parse=nil`**；字段不存在只在 `Execute`
时报错。实现所述完全正确。

需要区分清楚的是：**规则 1 的结论仍然成立，只是理由错了。** 结构体确实优于 `map[string]any`
（map 上未知字段静默成空值，结构体上是硬错误），这个取舍不变；错的是「该错误发生在解析期，
因而启动时可告警回落」这一步推导。T14 按现文字**不可实现**。

规划需要在 r2 里裁决：放弃启动期检测（把 T14 降为运行期，与 T15 合流并说明边界），
或补一个方案未描述的机制（静态遍历 parse tree 校验字段白名单 / 零值结构体预执行），
并明确它与 T15 运行期失败的覆盖边界。**同样是规划的裁决。**

## 4. 我的 Gate 也漏了这两条——机器核对覆盖不到语义互斥

闸门 1 我给了 PASS。现在看，`leader/20260826-111707-gate1-plan.md` 的核对做的是章节完整性、
`work_partition` 合法性、Council 提名是否引用具体 path、正反验收是否**存在**——
**没有一项会去检查「不变量 A 与验收项 B 是否可以同时为真」**。

两处 finding 的共同形态：**方案在不同章节各自正确，交叉处不成立。**
INV2（§5）与 T08（§8）分开读都对；§7.1 规则 1 的取舍对、理由错。
这类冲突只有在**逐条设计 RED 时**才会暴露——实现正是在准备 harness、逐条设计 T08/T14 的
RED 时撞上的，这恰好是流程期望它被发现的位置。

对我的 Gate 的具体修正，登记在案：

> **方案 Gate 增加一项语义核对：把「安全不变量」与「逐字节/逐值验收项」两两对照，
> 凡验收项断言了某个具体字节/错误时机的，必须回到不变量规定的处理管线里确认该断言可达。**
> 本次两条都属于这一类：T08 断言了具体字节 `=0D=0A`，T14 断言了具体时机「解析期」。
> 断言越具体，越要回管线验证——它们不是"更严格"，而是**更容易与上游处理步骤打架**。

这一条与我此前四次「未复算的断言」是同一根：**方案里任何一句"实测…"的括号，
在被升格为验收项之前，必须确认它所描述的那次实测与最终管线的输入相同。**
§7.1 那个 `=0D=0A` 括号就是在管线的**上游**测的，被 T08 拿到管线**下游**当预期用。

## 5. 路由裁决

- 冻结方案 `planner/20260826-110423-plan-r1.md` **解冻**，subject tree `37fb17c8…` 不再是候选基准。
- **新建规划卡片，沿用原 stage 号 1**（HD-52 已 `done`，按角色约束不复用已完成卡片）。
  范围严格限定为 PG1、PG2 两处修订 + 全文自洽性复查，**不重开方案 A/B/C 路线选择**——
  两处 finding 都不触及路线选择，A 的论证依据（`auth.go:334-360` 源码事实）未被动摇。
- HD-53 保持 `blocked`，等 r2 冻结并过闸门 1 后由我提升。**不新建实现卡片**——
  实现零产出、零 diff，原卡片可原地复用。
- HD-54（评审，stage 3）保持 `backlog`，不受影响。
- 父 Issue HD-51 保持 `in_progress`。**本次不进人工闸门**：方法正本规定核对 RETURN 时按 finding
  路由返工、不进闸门。r2 冻结后走完整闸门 1（含 @ 发起人）。

## 6. 环境事实（转给规划与后续实现，非 finding）

实现记录的两条容器差异，我在本容器复算确认同样成立，已写进新卡片：

- `PATH` 不含 `/usr/local/go/bin`，须显式 `export PATH=$PATH:/usr/local/go/bin`。
- `TMPDIR` 指向 `noexec` 的 `/tmp`，Go 测试二进制会 `fork/exec: permission denied`；
  须显式设 `GOTMPDIR` 到 `/workspaces` 下。

这两条不构成方案 finding，但会让任何 Go 命令在首次执行时失败，属于必须前置告知的工作区事实。

## 7. 交接

- 状态：PLANNING（由 IMPLEMENTING 退回）
- 产物：本分录；方案 r1 解冻
- 下一步：规划出 r2 修订 PG1/PG2 并全文自洽复查 → 我做核对 → 闸门 1 → 提升 HD-53
- 未解决：PG1 的安全契约取向、PG2 的加载期校验机制与 T15 边界，均待规划裁决
- 需要决策：无（均已路由）
