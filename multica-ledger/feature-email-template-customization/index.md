# feature/email-template-customization

- Issue：HD-51（`01a03daf-fbf6-71d3-ac72-1f4fea15be09`）— 为自建部署提供对外邮件模板自定义能力（上游 #7569）
- 仓库：`https://github.com/xiaoyaner0201/multica`（`origin` = fork，`upstream` = `multica-ai/multica`）
- Base：`f74a71060fcdd45318cae6fb7caf7c0d7e71cdfa`（live `upstream/main` @ 2026-08-26，tree `57e81be90cc49effdc55aa2f06f16aef35689aba`）
- 风险分级：`HIGH_RISK`
- 冻结计划：`planner/20260826-134000-plan-r3.md`（sha256 `025d33e6…b342ac`），于 20260826-145500
  核对 **PASS**，裁决见 `leader/20260826-145500-gate-plan-r3.md`；于 20260826-150500 经**人工闸门 1
  放行**（发起人 dongsjoa，附两条追加约束），见 `leader/20260826-150500-gate1-approved.md`。
  r3 已于 20260826-152000 因 **PG16 解冻**，见 `leader/20260826-152000-gate-pg16-route.md`。
  r2（`…-121052-plan-r2.md`）于 20260826-123504 核对 RETURN（PG10/PG11）而**从未冻结**；
  r1（`…-110423-plan-r1.md`）已于 20260826-115806 解冻。账本 append-only，四份均原样保留。
  头部「冻结计划」一行只反映**已过 Gate 的事实**，不反映最新产物（见 `…-123504-gate-plan-r2.md` §5 M-L4）
- **待放行方案**：`planner/20260826-154500-plan-r4.md`（sha256 `4e7fef61…4ce0e`，
  `server_tree` = `df467331`），FROZEN，于 20260826-161500 核对 **PASS**
  （裁决见 `leader/20260826-161500-gate-plan-r4.md`，10 项事实 + 4 组数字 + 3 条代码事实 + 8 项语义判据全部复现，
  新增非阻塞 **PG20**），等**只呈 PG16 + PG20** 的人工闸门 1。**核对通过 ≠ 可推进**。
  r4 = r3 + PG16 裁决 + 既有约束就地落地，**未重开已放行方向**；
  「不是重写」有机械证据：r4 先以逐字节 `cp` r3 单独提交（`2d0c1a9b7`），
  之后每处修改一个 commit，`git diff 2d0c1a9b7 HEAD -- <r4>` 即完整且唯一的修改集。
- r4 对四项未决约束的处置：
  - **PG16**（曾阻断，**已裁决**）：选**出路一——删除 `verificationTemplateData.AppURL`、
    维持硬规则**。决定性理由是能力增量为零（`AppURL` 是部署期常量，写模板的运维方本就持有，
    与服务端现生成的 `Code` 有本质区别），另有可逆性、凭据类邮件最小化、判据存活三条支撑。
    硬规则第 3 条已重写为单一可机械执行判据：字段 F 准入当且仅当存在确定性 `g`，使
    `F = g(该封信在 BASE 上已产生的 Subject 与 HTML 正文字节)`——逐封信而非逐代码库，
    因此正确放行邀请模板的 `InviteURL` 与 `AppURL`，同时拒绝验证码侧的 `AppURL`。
    **仍需人工闸门确认**（授权面类决定不归规划终裁），闸门正文只呈这一项。
  - **追加约束一**：已在 r4 §14 点名责任与三点要求；**交付物仍是实现的
    `coder/<ts>-publication-notes.md`**，不在方案产物内。
  - **追加约束二**：已落为 r4 **§5.2「为什么是截断而不是折行」**独立一节
    （folding 是 RFC 5322 §2.2.3 正解、可行、不做的三条理由、同时能修内置 1349 越界、
    以及本节在什么情况下是错的）。
  - **PG15**：选**删除**不可复算的 `71` / `151`（§0.4、§5.1、§12 R2 三处），
    布尔结论改挂确定性枚举探针 G / B5；文档面数字白名单 91 / 99 / 200 / 67 写进 T21。
- r4 逐元素自查（M-L18 的自查侧执行）新增三处并各自裁决：**PG17** 主题模板可放 `{{.Code}}`
  → 允许但 T21 四语言必须警告；**PG18** `ExpiresInMinutes` 双字面量 → 改从 `service` 包常量取值
  并加 T27，不修 `auth.go` 跨包耦合（列为 U10）；**PG19** DEV 模式模板是否生效（PG16 同型缺陷）
  → 裁定 DEV 分支渲染前返回，stdout 与 BASE 逐字相同。
  另按 M-L19 重扫全部 `must_answer`，**命中 7 条**全部改写为可证伪探针。
- **PG20**（新，20260826-161500 调度核对时发现，**非阻塞**）：`AppName` 通过了字段准入判据，
  却没通过 §0.8.3 用来删掉 `AppURL` 的那条「能力增量为零」决定性判据，r4 未把两者放在一起比较；
  且 r4 全文无 `AppName` 赋值处、未新增任何品牌名配置项，其值由 `g` 隐式确定为字面量 `Multica`
  ——自建方在模板里写 `{{.AppName}}` 会渲染出上游品牌名。三条出路（删 / 留但四语言文档写明 /
  留并新增配置项但须显式修订判据）中两条是产品取向，带进闸门 1 由发起人裁决，实现不代决。
- 当前状态：PLAN_GATE1（r4 核对 PASS，等待只呈 PG16 + PG20 的人工闸门 1 放行）
- 卡片：HD-52【方案 r1】stage 1（done）· HD-55【方案 r2】stage 1（done）· HD-56【方案 r3】stage 1（done）· **HD-57【方案 r4】stage 1** · HD-53【实现】stage 2（blocked）· HD-54【评审】stage 3（backlog）
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
| 20260826-150500 | leader | 闸门 1 放行（+2 条追加约束） | leader/20260826-150500-gate1-approved.md | `51feb441` | APPROVED |
| 20260826-152000 | leader | PG16 路由（4 项证据独立复算）→ r3 解冻，退规划出 r4 | leader/20260826-152000-gate-pg16-route.md | `a9bfbabc` | RETURN（PG16 阻断） |
| 20260826-154500 | planner | plan r4（裁决 PG16 + 判据重写 + 逐元素回归 PG17–PG19 + 追加约束二成节 + PG15） | planner/20260826-154500-plan-r4.md | `641a2b12` | FROZEN |
| 20260826-161500 | leader | gate 方案 r4（10 项事实 + 4 组数字 + 3 条代码事实 + 8 项语义判据独立复算） | leader/20260826-161500-gate-plan-r4.md | `641a2b12` | PASS（新增 PG20，非阻塞） |
