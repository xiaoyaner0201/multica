---
ts: 20260826-072439
agent: leader
action: Discovery 前置校验 + Plan dispatch
tree: dbbdd022f1895e6c694f2c6f75f2c9439f0e987d
verdict: PASS
---

## 迟滞说明（先记，因为它是本 Issue 的真实事实）

HD-49 于 `2026-08-26T06:58:23Z` 置 `in_review`，本分录 `07:24` 才产生。中间约 26 分钟没有任何
Gate 或 dispatch 动作，**不是任何工程原因**：Stage 1 完成后没有 Run 唤醒调度，直到老板在父
Issue 上 mention 才触发本 Run。Discovery 产物在这段时间内一直是合格且可用的。

记在账本里是因为这属于编排面的可复发缺陷，不是一次性偶然：本协议依赖调度在每个 Stage
barrier 上做机械 Gate，而 barrier 目前不保证回唤调度。缓解见文末。

## Discovery 前置校验（可在 Stage 1 校验的部分）

对 `hd-47/smtp-from-mailbox` @ `0cd73c1e44fe389b3f24bd14eb7994ee6d24aa71` 实跑：

| 检查 | 结果 |
|---|---|
| inventory sha256 与 Coder 声明一致 | PASS `b4d374f29489178f4fa90faa9df74abfb40db85464963b7c8c13d16a29670491` |
| commit / tree 与声明一致 | PASS `0cd73c1e…` / `dbbdd022f1895e6c694f2c6f75f2c9439f0e987d` |
| `schema_version` | PASS `behavior-inventory.v1` |
| `subject.base_commit` / `base_tree` 与 index 头部一致 | PASS `620f99d8ce16…` / `9fb8974cdc40…` |
| `maturity` ≠ `D0` | PASS，为 `D1` → **强制 warning 生效** |
| `produced_by_run_id` | `8c198cbd-edf5-4a57-8936-4c42456c9b50` |
| `produced_by_context_id` | `agent:fd2d1252-…/container-slot:4c42456c9b50/issue:8d1cabf6-…` |
| candidate 数 / ID 唯一 | PASS 43 / 43 |
| `must_fail_old=true` | 9 条 |
| 业务面漂移 `git diff BASE..HEAD -- . ':(exclude)multica-ledger'` | PASS 空 |
| 工作树 clean | PASS |

`produced_by_*` 与 Plan Run 的比对**无法在本 Stage 完成**——Plan Run 尚不存在。该检查由
`run_gate.sh --kind plan` 在 Stage 2 收线时执行，本分录不预先宣称其通过。Coder 声明的
provenance 值已如实抄录于上，供该次比对使用。

同样如实记录：`run_gate.sh` 无 `--kind inventory` 模式（`choices=["plan","qa","review"]`），
三种模式均要求 Stage 1 尚不存在的 artifact，因此**本 Stage 没有跑过完整 gate，也没有生成
gate receipt**。Coder 复刻了 `validate_gate.py:514-522` 的 inventory 检查段并提交脚本与输出；
调度独立复核了上表各项。这是必要条件，不是 Plan Gate 的替代。

## Discovery 质量认定（语义部分，非机械）

三点超出 dispatch 要求且实质降低了后续漏检风险，记账以便 Reviewer 与后续 Stage 复用：

1. **C13/C31 头注入面**被识别为**位置性防护**而非内在防护——`email.go:319` 自身无 CR/LF
   校验，今天不可达仅因 `c.Mail`（L309）先触发 stdlib `validateLine`。父 Issue 未提此面。
   Coder 如实标 `must_fail_old=false` 并说明「今天一个断言无注入的探针会通过，因此不是合法
   RED」——这是正确的诚实，而非覆盖缺口。
2. **C39** 认定现有 mock server 不保存 `MAIL FROM:` 行（`email_test.go:610-611`）、DATA 逐行
   丢弃（`616-625`），三个相关测试**在信封完全畸形时也会通过**。这直接命中父 Issue 给 QA 的
   盲区警告，且把 harness 改造变成 Plan 必须计入的工作量。
3. **C22 假阳性排除**：`email.go:376` 在 SMTP 分支调 `buildInvitationParams(s.fromEmail, …)`
   但返回的 `.From` 被丢弃；只看 grep 会误建模为第三个发射点。

以及一个与父 Issue 措辞的实质落差，已转为 Plan 决策点：whitespace-only 取值在 L122 被 trim
成 `""` 后**回退到 `RESEND_FROM_EMAIL`**，并不走 L259 报错。父 Issue 不变量 2 写的是「空值保持
现有错误」，但现有行为在该输入上是回退。

## 结论

Discovery `PASS`，inventory **FROZEN**。HD-49 收线。

D1 warning 随本分录落账并向下传递：**inventory 不构成行为面完整性上界**。Planner 须独立建模，
Reviewer 的 `BLIND_BEHAVIOR_AUDIT` 不得以「inventory 没列」推出「系统里没有」。

## Plan 派单

Stage 2 → HD-50，指派「规划」。dispatch 中点名四个 Discovery 刻意未收敛的决策点（C28 双向
兼容冲突、C30 多 mailbox 语义、whitespace-only 的不变量 2 落差、startup vs send-time 校验），
两项必须计入的工作量（C13/C31 防护设计、C39 harness 改造），以及 C20/C21 因带
`INV6-resend-unaffected` 而**不得以 non-goal 排除**的 validator 约束。

要求产出 `work_partition` 与 `council_nomination`。已提示：本改动写集高度集中于单文件加其
测试，若判断不该拆则输出单一单元并说明理由，不要为并行而并行。

## 缓解：barrier 不回唤调度

在本 Issue 剩余 Stage 上，调度不依赖 barrier 自动唤醒。每次派单的 dispatch 正文已要求下游
Agent 完成后置 `in_review`；调度改为在**派单的同一 Run 内**记录下一个待检查点，并接受由
mention 或后续 Run 触发的回收。若再次出现 Stage 完成后长时间无人推进，属同一编排面缺陷，
应作为独立 Issue 提给控制面，而不是在本 Issue 内反复人工兜底。

## 交接

- 状态：PLANNING（Stage 2 进行中）
- 产物：inventory FROZEN；HD-49 收线；HD-50 已派
- 下一步：Planner 产出 `plan-r1` + plan-gate artifact + `work_partition` + `council_nomination`；
  调度跑 `run_gate.sh --kind plan --artifact <plan> --inventory <本 inventory>`，届时才执行
  `produced_by_run_id` / `produced_by_context_id` ≠ Plan Run/context 的机械比对
- 未解决：6 条 `UNVERIFIED`（C14/C16/C35/C41/C42/C43），其中 C35/C41/C43 经 Coder 判断会实际
  影响 Plan 决策；Council 名单待提名
- 需要决策：无（Plan Gate 审批按 Project description 路由，届时触发）
