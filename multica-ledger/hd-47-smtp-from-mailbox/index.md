# hd-47/smtp-from-mailbox

- Issue：HD-47 `b9e8d88d-45b3-4d6c-a832-cb22decafb88` — fix(email): parse SMTP_FROM_EMAIL so envelope and From header each get the correct form
- 上游 Issue：https://github.com/multica-ai/multica/issues/7568
- Repo：https://github.com/xiaoyaner0201/multica （fork of multica-ai/multica）
- Base：`620f99d8ce168ee56bfcfacf359b3bcca30f21bf`（live upstream/main，2026-08-26 重新读回）
- Base tree：`9fb8974cdc4038f9c42a99f3fbf1613729f89def`
- 风险分级：`HIGH_RISK`
- 冻结 inventory：`coder/20260826-064855-behavior-inventory.json`（sha256 `b4d374f2…0491`，maturity **D1**，43 candidates，6 UNVERIFIED）— FROZEN，不得修改
- 冻结计划：（待 Planner 产出）
- 当前状态：PLANNING

## 分录

| 时间 | Agent | 动作 | 产物 | tree | 结论 |
|---|---|---|---|---|---|
| 20260826-063943 | leader | intake+dispatch | leader/20260826-063943-intake-dispatch.md | — | — |
| 20260826-064855 | coder | BEHAVIOR_DISCOVERY | coder/20260826-064855-behavior-discovery.md | — | FROZEN |
| 20260826-072439 | leader | discovery 校验+plan dispatch | leader/20260826-072439-discovery-accept-plan-dispatch.md | `dbbdd02` | PASS |
