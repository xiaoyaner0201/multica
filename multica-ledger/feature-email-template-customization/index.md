# feature/email-template-customization

- Issue：HD-51（`01a03daf-fbf6-71d3-ac72-1f4fea15be09`）— 为自建部署提供对外邮件模板自定义能力（上游 #7569）
- 仓库：`https://github.com/xiaoyaner0201/multica`（`origin` = fork，`upstream` = `multica-ai/multica`）
- Base：`f74a71060fcdd45318cae6fb7caf7c0d7e71cdfa`（live `upstream/main` @ 2026-08-26，tree `57e81be90cc49effdc55aa2f06f16aef35689aba`）
- 风险分级：`HIGH_RISK`
- **冻结计划（当前执行依据）**：`planner/20260826-154500-plan-r4.md`
  （sha256 `4e7fef61…4ce0e`，`server_tree` = `df467331`），于 20260826-161500 核对 **PASS**
  （`leader/20260826-161500-gate-plan-r4.md`，10 项事实 + 4 组数字 + 3 条代码事实 + 8 项语义判据全部复现，
  新增非阻塞 **PG20**），于 20260826-163000 经**人工闸门 1 放行**（发起人 dongsjoa），
  见 `leader/20260826-163000-gate1-approved-r4.md`。
- ⚠️ **执行依据 = 冻结方案 r4 + 全部放行分录的追加约束**，不是 r4 单独一份。
  r4 冻结后**不重走 stage 1**，因此闸门 1 第二次放行新增的**追加约束三/四/五只存在于
  `leader/20260826-163000-gate1-approved-r4.md` §2**（含落点与检查方对照表）。
  实现开工、评审对照，均须一并纳入。见该分录 §6 M-L25。
- 历史版本（append-only 原样保留，均**不再是执行依据**）：
  r3（`…-134000-plan-r3.md`）20260826-145500 核对 PASS、150500 过闸门 1，152000 因 **PG16 解冻**；
  r2（`…-121052-plan-r2.md`）20260826-123504 核对 RETURN（PG10/PG11），**从未冻结**；
  r1（`…-110423-plan-r1.md`）20260826-115806 解冻。
  头部「冻结计划」一行只反映**已过 Gate 的事实**（见 `…-123504-gate-plan-r2.md` §5 M-L4）。
- r4 = r3 + PG16 裁决 + 既有约束就地落地，**未重开已放行方向**；
  「不是重写」有机械证据：r4 先以逐字节 `cp` r3 单独提交（`2d0c1a9b7`），
  之后每处修改一个 commit，`git diff 2d0c1a9b7 HEAD -- <r4>` 即完整且唯一的修改集。
  该做法在 20260826-161500 核对中把「未重开已放行方向」从通读全文降为一条命令 + 43 个 hunk，
  建议写进正本（见该分录 §6 M-L22）。
- r4 对四项未决约束的处置：
  - **PG16**（曾阻断，**已裁决**）：选**出路一——删除 `verificationTemplateData.AppURL`、
    维持硬规则**。决定性理由是能力增量为零（`AppURL` 是部署期常量，写模板的运维方本就持有，
    与服务端现生成的 `Code` 有本质区别），另有可逆性、凭据类邮件最小化、判据存活三条支撑。
    硬规则第 3 条已重写为单一可机械执行判据：字段 F 准入当且仅当存在确定性 `g`，使
    `F = g(该封信在 BASE 上已产生的 Subject 与 HTML 正文字节)`——逐封信而非逐代码库，
    因此正确放行邀请模板的 `InviteURL` 与 `AppURL`，同时拒绝验证码侧的 `AppURL`。
    20260826-163000 **闸门 1 照准**：放行人独立复核 BASE 后确认两条规定无法同真、
    「能力增量为零」是决定性理由，并明确接受代价（换域名时验证码模板链接要自己改，域名两处维护）。
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
  ——自建方在模板里写 `{{.AppName}}` 会渲染出上游品牌名。
  20260826-163000 **闸门 1 裁决：选 (b)——保留 `AppName`，但四语言文档必须写明它固定渲染
  `Multica`、要用自己的品牌请直接在模板里写字面量**（追加约束三），并在 PR 描述主动交代该取舍
  （追加约束四）。不选删除是因为会失去与内置主题保持一致的引用方式；不选新增配置项是因为
  它违反 r4 刚立的判据——**刚立的判据第一次遇到不便就修订，判据就没有约束力了**。
- **追加约束五**（闸门 1 第二次）：PR 描述须一句话点出 `FRONTEND_ORIGIN` 为空时兜底为硬编码
  `https://multica.ai`（`email.go:370-372`）是**既有行为、本次未改**，避免自建方误判为本功能 bug。
- **流程修订**（20260826-163000 放行人当场宣布）：实现期发现的方案缺陷若影响半径是**局部的**
  （字段取舍、判据歧义、局部矛盾），**就地裁决后由规划补修订分录，实现在原卡片继续，
  不再新建方案卡片重走整轮**。PG16 属此类，故本次不新建方案卡片，HD-53 直接恢复。
- **T19 / DB 面 = 携带进 stage 3 的两项 `UNVERIFIED`**（20260826-174840 调度独立复算裁决，
  见 `leader/20260826-174840-gate-t19-evidence-gap.md`）：
  - **T19**（全仓 `scripts/test-go.sh --race` 全绿）：本容器不可得，且**与本次改动无关**。
    5 个失败包（`cmd/multica`、`internal/cli`、`internal/daemon`、`internal/daemon/execenv`、
    `pkg/agent`）对 BASE 逐字节**零 diff**，失败测试集合在 BASE 与 candidate 上一致；
    承载改动的 `internal/service` race **PASS**（58 ok / 5 FAIL / 63 包）。
    验证归因的 task marker **经复算不成立**（六种变量组合单独排除后仍 FAIL）。
    根因是节点拓扑：exec 能力（仅 `/workspaces`）与 marker-free（其余 tmpfs，均 noexec）**不相交**，
    故**任何 agent 容器都不能签发 T19**，证据须来自真实 CI。
  - **DB 面**：本容器无 PostgreSQL 服务端（仅客户端二进制，无 docker，`DATABASE_URL` 未设）。
    `internal/handler` 等包在 TestMain 层整包跳过却仍打印 `ok`（实测 `--- PASS` 计数为 0）。
    **58 个 `ok` 不得读作全仓 DB 覆盖通过。**
  - 两项均**不得写成 PASS**，由评审如实列入四维结论的 `UNVERIFIED`，在**闸门 2 由决策者裁定**
    是否须补真实 CI 结果后再判 READY。裁决权不在评审，也不在调度。
- **闸门 2（交付审批）= 当前所在位置，结论 `RETURN`，不判 READY**
  （20260826-183500 调度独立核对，见 `leader/20260826-183500-gate2-return.md`）：
  - 事实层 12 项全部复算一致；评审引用的每个 digest 都对得上。
  - **开放 high `F1`：空 / 纯空白 / 仅注释的模板文件被判为渲染成功，产出空邮件，
    不告警、不回落，HTTP 仍回 200——自建部署的唯一登录通道静默失效。**
    我在仓库外独立复现（`Parse`/零值 `diagnose`/运行期 `Execute` 三环节全部无错），
    并把影响面扩大两处：**(a) SMTP 通道同样无守卫**（评审只在 Resend 侧复现过）；
    **(b) 只空一个文件即触发半空形态**（主题正常、正文无码，比两个都空更隐蔽）。
  - `F2`（medium，`INVARIANT_VIOLATION`）：`maxSubjectFieldRunes = 60` 的逐字段钓鱼额度
    在模板主题路径上不生效，跨租户可控的 `WorkspaceName` 放宽到 91（CJK）/ 200（ASCII）。
  - `F3`（medium，`PLAN_GAP`）：§7.2 的挂载目录信任边界结论未进任何交付文档（我全量 grep 复核）。
  - **返工去向三条全部指向规划**，判为**增量修订不是重写**；**当前不路由到验证**
    （判据未定前补探针＝把裁决权推给写测试的人）。
  - Pass 1 → Pass 2 的两条撤销（F4 TTL 双字面量、F5 内置主题 1349 octet）**照准**。
  - **本轮进闸门而非直接派返工**，因为有三项裁决权只在决策者：T19/DB 面是否须补真实 CI、
    U1 上游取向不可验（评审 L3-2 另指出「只把 DB+UI 限定在邀请邮件上，方案 C 重新可行」）、
    F2 的范围取舍。**不起返工卡片**，等决策者裁定，避免替它预设「要再做一轮」。
  - M-L25 实测：追加约束三/四/五**全部被评审检查到**（其 §0 把 gate1 分录列为必读、§1.7 逐条 PASS），
    预测的静默失效未发生；但这是派单钉指针的功劳，不是正本规定了对照基准，**建议维持**。
- 当前状态：**GATE 2 — `RETURN`，父 Issue `in_review` 待决策者裁决**
- 卡片：HD-52【方案 r1】stage 1（done）· HD-55【方案 r2】stage 1（done）· HD-56【方案 r3】stage 1（done）· HD-57【方案 r4】stage 1（done）· HD-53【实现→验证】stage 2（done）· HD-54【评审】stage 3（done，Pass 1 盲审 + Pass 2 证据挑战，四维结论 RETURN）
- 闸门：闸门 1 方案审批（20260826-163000 已放行）/ 闸门 2 交付审批（20260826-183500 核对 RETURN，待裁决），均在父 Issue HD-51 上，@ 发起人 dongsjoa

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
| 20260826-163000 | leader | 闸门 1 放行 r4（PG16 照准 + PG20 选 (b)，+3 条追加约束，流程修订） | leader/20260826-163000-gate1-approved-r4.md | `34631e84` | APPROVED |
| 20260826-164006 | coder | U1 publication notes | coder/20260826-164006-U1-publication-notes.md | `ecc74e58` | — |
| 20260826-164100 | coder | U1 RED→GREEN | coder/20260826-164100-U1-code.md | `ecc74e58` | CANDIDATE |
| 20260826-165358 | qa | U1 independent charter | qa/20260826-165358-U1-charter.md | `ecc74e58` | — |
| 20260826-171625 | qa | U1 independent verification | qa/20260826-171625-U1-verification.md | `ecc74e58` | RETURN |
| 20260826-174840 | leader | T19 EVIDENCE_GAP 路由（7 项数字复算 + 全仓 race 63 包 + BASE 对照 + 6 项假设排除 + 新查 DB 缺口） | leader/20260826-174840-gate-t19-evidence-gap.md | `ecc74e58` | CARRY_UNVERIFIED（不返工，携带进 stage 3） |
| 20260826-180154 | reviewer | Pass 1 盲审重建（16 项行为图 + 19 探针，含 SMTP 线级）| reviewer/20260826-180154-blind-audit.md | `ecc74e58` | FROZEN（F1 high / F2 medium，四维待 Pass 2）|
| 20260826-181735 | reviewer | Pass 2 证据挑战 + 四维结论（5 项证据挑战 + Council 三 lens 共 22 问 + 追加约束三/四/五 核查）| reviewer/20260826-181735-verdict.md | `ecc74e58` | **RETURN**（F1 high PLAN_GAP / F2 medium INVARIANT_VIOLATION+SECURITY / F3 medium PLAN_GAP；Intent·Invariant·Evidence FAIL，Exact Tree PASS；U1 + T19 + DB 面 UNVERIFIED 原样携带）|
| 20260826-183500 | leader | **闸门 2 核对**（12 项事实复算 + F1 仓库外独立复现并扩大影响面两处 + F2/F3 代码与文档面复核 + 5 项语义判据 + 返工路由照准 + M-L25 实测）| leader/20260826-183500-gate2-return.md | `ecc74e58` | **RETURN**（开放 high F1，不判 READY；三条 finding 全部去向规划，判为增量修订；进闸门而非直接派返工，因 T19/DB、U1、F2 范围三项裁决权只在决策者）|
