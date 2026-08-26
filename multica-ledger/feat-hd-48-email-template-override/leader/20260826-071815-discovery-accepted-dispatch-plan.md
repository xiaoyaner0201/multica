---
ts: 20260826-071815
agent: leader
action: discovery provenance check + dispatch Plan
tree: d557983da60691281813ffc3580debb635197887
verdict: PASS
---

## 机械校验：`behavior-inventory.v1` 接收

对 `coder/behavior-inventory.v1.json`（commit `3ddde1889`，tree
`d557983da60691281813ffc3580debb635197887`）逐项复算，全部由我独立执行，不采信报告文字：

| 检查 | 结果 |
|---|---|
| sha256 与申报一致 | PASS `fbc4604be3ddaf6d254a98ff3d5fb33e45fee56bd06cd5b4af8351fccdcbc8ee` |
| schema 合规 | PASS（独立跑 `schemas/behavior-inventory.schema.json`，非采信 coder 输出） |
| candidate 数与 id 唯一性 | PASS 24 条，id 无重复 |
| `maturity` ≠ `D0` | PASS `D1` |
| provenance 非占位符 | PASS run `02cc6075-02d4-448e-8225-25158762e98b` / context `25158762e98b` |
| provenance ≠ Plan Run | 延后：Plan Run 尚不存在，由 `run_gate.sh --kind plan` 强制 |
| 业务路径零漂移 | PASS `git diff 1ef1d65be..HEAD -- . ':(exclude)multica-ledger/**'` 空 |
| ledger append-only | PASS 该 commit 仅新增文件 + `index.md` 表尾一行；未改头部、未改既有分录 |

接收。

## 口径更正：D1 正确，我的 dispatch 措辞错误

我在 `leader/20260826-065534-dispatch.md` 的派发评论中写「能给出确定性可复现命令就标
`D2`」。**这是错的，本分录更正之。** skill 正本对 D2 的定义是「AST / Semgrep / CodeQL，
跨文件解析调用与注解」——判据是**是否解析调用**，不是命令是否可复现。coder 的发现器用
`go/ast` 跨 1555 个文件解析、输出确定性排序，但按 selector 名字匹配调用目标、无
`go/types`（本环境 module cache 无 `golang.org/x/tools`，离线拿不到 `go/packages`），
名字匹配够不到「解析调用」，因此 `D1` 是正确标注。

coder 按「与 dispatch 矛盾时按 skill 执行并报告矛盾」处理，处置正确。缺陷在我：我的
措辞把 skill 的判据替换成了一个更松的判据，等于自行放宽 Gate 判据，而这正是角色绑定
明令禁止的。后续 dispatch 不再复述 skill 判据，只引用。

后果按协议保留：Plan Gate 将对 `D1` 打强制 warning，inventory **不是完整性上界**，
评审的 `BLIND_BEHAVIOR_AUDIT` 必须独立追踪，不得以「inventory 没列」作为不存在的依据。
对一个改写登录凭据邮件的任务，这条 warning 是正确信号，不予豁免。

## 独立复核的五条源码事实

coder 报告中会决定 Plan 形状的五条，我逐条自行 grep 复核，全部成立：

1. `internal/service/email.go:177` — `func NewEmailService() *EmailService`，无 error 返回；
2. `internal/handler/invitation.go:239-243` — detached goroutine，错误仅 `slog.Warn`；
3. `internal/service/email.go:409` — body 实参 `(safeWorkspace, safeInviter, safeWorkspace, inviteURL)`，位置插值；
4. `internal/service/email_test.go:616-625` — `testSMTPServer` DATA 分支逐行丢弃；全树 `Subject:` 断言计数 **0**；
5. `SendVerificationCode` 在 `*_test.go` 中引用计数 **0**。

一处修正 coder 的表述：其称 `cmd/server/router.go` 为「唯一构造点」。生产构造点确为唯一
（`cmd/server/router.go:421`），但另有三处测试构造点
（`internal/handler/handler_test.go:66`、`internal/service/email_test.go:158/185/245`）。
若 Planner 选择改签名，这三处进写集，`work_partition` 必须体现。

## 派发：Planner

派 `规划` 产出 `plan-r1` 与 plan-gate artifact，对 24 条 candidate 各恰好一次
disposition。三条 UNVERIFIED 与 `DEPLOY-ENV-PLUMBING` 不得仅以 `non-goal` 排除
（其中触及授权/隐私不变量者由 validator 强制拒绝该排除方式）。

需 Planner 显式裁决、不得默认的四处（详见派发评论）：不变量 4 与
`CTOR-NEWEMAILSERVICE-STARTUP` 的架构冲突；CJK/byte-identity 验收在当前 harness 下
不可观测；位置插值 vs 具名占位符；DEV sink 是否进模板化范围。

## 交接

- 状态：PLANNING，discovery 已冻结接收
- 产物：inventory `coder/behavior-inventory.v1.json`，digest 见上，tree `d557983da`
- 下一步：规划出 `plan-r1` + plan-gate artifact（含 `work_partition` 与 `council_nomination`）
- 未解决：三条 UNVERIFIED 候选待 Planner 建模
- 需要决策：Plan Gate 的人类审批按 Project description 路由，待 artifact 齐备后由我发起
