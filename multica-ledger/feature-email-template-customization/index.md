# feature/email-template-customization

- Issue：HD-51（`01a03daf-fbf6-71d3-ac72-1f4fea15be09`）— 为自建部署提供对外邮件模板自定义能力（上游 #7569）
- 仓库：`https://github.com/xiaoyaner0201/multica`（`origin` = fork，`upstream` = `multica-ai/multica`）
- Base：`f74a71060fcdd45318cae6fb7caf7c0d7e71cdfa`（live `upstream/main` @ 2026-08-26，tree `57e81be90cc49effdc55aa2f06f16aef35689aba`）
- 风险分级：`HIGH_RISK`
- 冻结计划：`planner/20260826-134000-plan-r3.md`（sha256 `025d33e6…b342ac`），于 20260826-145500
  核对 **PASS**，裁决见 `leader/20260826-145500-gate-plan-r3.md`。**核对通过 ≠ 可推进**：
  尚待人工闸门 1 放行。r2（`…-121052-plan-r2.md`）于 20260826-123504 核对 RETURN（PG10/PG11）
  而**从未冻结**；r1（`…-110423-plan-r1.md`）已于 20260826-115806 解冻。账本 append-only，三份均原样保留。
  头部「冻结计划」一行只反映**已过 Gate 的事实**，不反映最新产物（见 `…-123504-gate-plan-r2.md` §5 M-L4）
- 未决约束：**PG15**（`…-145500-gate-plan-r3.md` §4）——方案 §0.4/§5.1 中 emoji 混排 `71`
  与 `mixed 151` 两个数字构造未记录、不可复算，非阻塞，作为约束带进 stage 2：
  文档面只许用 91 / 99 / 200 / 67
- 当前状态：PLAN_GATE（方案 r3 核对 PASS，等待闸门 1；第 2 轮返工完成）
- 卡片：HD-52【方案 r1】stage 1（done）· HD-55【方案 r2】stage 1（done）· **HD-56【方案 r3】stage 1** · HD-53【实现】stage 2（blocked）· HD-54【评审】stage 3
- 闸门：闸门 1 方案审批 / 闸门 2 交付审批，均在父 Issue HD-51 上由调度执行，@ 发起人 dongsjoa

## 分录

| 时间 | Agent | 动作 | 产物 | tree | 结论 |
|---|---|---|---|---|---|
| 20260826-105522 | leader | intake+dispatch | leader/20260826-105522-dispatch.md | — | — |
| 20260826-110423 | planner | plan r1 | planner/20260826-110423-plan-r1.md | — | FROZEN |
| 20260826-111707 | leader | gate 方案 | leader/20260826-111707-gate1-plan.md | `37fb17c8` | PASS |
| 20260826-112827 | leader | 闸门 1 放行 | leader/20260826-112827-gate1-approved.md | `37fb17c8` | PASS |
| 20260826-113113 | leader | supersede U1 | leader/20260826-113113-supersede-u1.md | `37fb17c8` | — |
| 20260826-113307 | leader | supersede 工作区契约 | leader/20260826-113307-supersede-workspace-contract.md | `01d9e4fd` | — |
| 20260826-114044 | leader | supersede 身份校验 r2 | leader/20260826-114044-supersede-identity-check-r2.md | `01d9e4fd` | — |
| 20260826-115329 | coder | PLAN_GAP | coder/20260826-115329-plan-gap.md | `e7c2d563` | RETURN |
| 20260826-115806 | leader | gate PLAN_GAP 路由 | leader/20260826-115806-gate-plan-gap-route.md | `37fb17c8` | RETURN |
| 20260826-121052 | planner | plan r2（裁决 PG1/PG2 + 全文自洽复查） | planner/20260826-121052-plan-r2.md | `7f9b0cc0` | FROZEN |
| 20260826-123504 | leader | gate 方案 r2 | leader/20260826-123504-gate-plan-r2.md | `5158dcd0` | RETURN（PG10/PG11） |
| 20260826-130731 | leader | HD-56 Run 失败复算 + 原地重跑 | leader/20260826-130731-rerun-hd56.md | `8664277c` | RERUN |
| 20260826-134000 | planner | plan r3（裁决 PG10/PG11 + 行号订正 + §8 数字全量复算） | planner/20260826-134000-plan-r3.md | `7f0664f6` | FROZEN |
| 20260826-145500 | leader | gate 方案 r3（16 项数字全量复算 + 7 项语义判据） | leader/20260826-145500-gate-plan-r3.md | `51feb441` | PASS（新增 PG15，非阻塞） |
| 20260826-145530 | coder | U1 PLAN_GAP r3（验证码模板 `AppURL` 白名单冲突） | coder/20260826-145530-U1-plan-gap-r3.md | `dc5a0f97` | RETURN（PG16） |
