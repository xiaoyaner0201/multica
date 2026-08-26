# feat/hd-48-email-template-override

- Issue：HD-48 `c44a65b8-3715-4841-9308-f8a9816b6ac7` — feat(email): allow self-hosted deployments to override outbound email templates
- 上游 Issue：https://github.com/multica-ai/multica/issues/7569
- Base：`1ef1d65bef2e2ecc2a2c04b2dcc99bc6a10f9e9c`（upstream/main，2026-08-26 重新同步；Issue 正文记录的 `8fda4f22d` 已过期）
- Base tree：`bfb2d1ef8bf8861cdadc3bdd6e1ce35f6bf48cee`
- 风险分级：`HIGH_RISK`（intake 机械判定，不可自行降级）
- 冻结计划：（待 Plan Gate PASS 后回填）
- 当前状态：PLANNING

## 分录

| 时间 | Agent | 动作 | 产物 | tree | 结论 |
|---|---|---|---|---|---|
| 20260826-065534 | leader | intake + dispatch discovery | leader/20260826-065534-dispatch.md | — | — |
| 20260826-070548 | coder | BEHAVIOR_DISCOVERY | coder/20260826-070548-behavior-discovery.md | — | FROZEN |
| 20260826-071815 | leader | discovery 接收 + 派 Plan | leader/20260826-071815-discovery-accepted-dispatch-plan.md | `d557983da` | PASS |
