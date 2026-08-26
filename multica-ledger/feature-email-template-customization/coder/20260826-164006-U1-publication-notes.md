---
ts: 20260826-164006
agent: coder
action: publication-notes
tree: ecc74e58
verdict: —
---

# U1 上游 PR 描述必带说明

绑定候选代码：commit `0a37dbada5bd9a6cd9f955252fec38631a7bf1cd`，tree
`ecc74e58c87472d9e9aa1722fdcae955428bab99`。

以下内容应主动写进上游 PR 描述；它们是已知取舍与既有行为，不应等 reviewer 追问后再解释。

## 1. 自定义路径与内置路径的 Subject 行上限不同

本次新增的自定义 Subject 路径会先剥离 `unicode.IsControl` 字符，再按 200 rune 截断，并保证
`Subject: ` 加 `mime.QEncoding` 后不超过 RFC 5322 的 998 octet 行上限。纯中日韩主题因此最多保留
91 rune（对应 996 octet）。

未配置 `EMAIL_TEMPLATE_DIR` 时，内置路径保持升级前的字节行为；本次没有把 998 octet 规则套到
内置路径。内置邀请主题在两个各 60 rune 的中日韩字段达到极端值时今天可产生 1349 octet 的
`Subject:` 行，**这是 BASE 已有问题，本次明确知道但未修**。原因是零配置升级的字节级兼容承诺：
改共享 `sendSMTP` 会同时改变没有启用模板的部署。

## 2. Header folding 是正确长期方案；本次截断是范围内取舍

RFC 5322 §2.2.3 的正确长期处置是 header folding，而不是丢字截断。Go 的 `mime.QEncoding` 已把
长主题拆成不超过 RFC 2047 单个 encoded-word 上限的词；在词间插入 `CRLF + WSP` 即可保留完整
语义并控制每行长度，技术上可行。

本次不做 folding，是因为它必须落在两条路径共用的 `sendSMTP` header 组装处，会改变内置邮件的
wire bytes，破坏零配置兼容不变量，并把「模板自定义」PR 扩成同时修既有 SMTP 合规问题。
**folding 也能同时修掉上节的内置 1349 octet 越界**，建议作为独立后续项交给上游。

## 3. `FRONTEND_ORIGIN` 的既有兜底没有改变

`FRONTEND_ORIGIN` 为空时，邀请链接仍按 BASE 逻辑兜底到硬编码 `https://multica.ai`
（候选代码 `server/internal/service/email.go` 的 `SendInvitationEmail`）。本 PR 没有修这处既有行为。
自建部署若希望邀请链接指向自己的实例，仍必须配置 `FRONTEND_ORIGIN`；应在 PR 描述点明，避免用户
把收到 multica.ai 链接误判为模板功能引入的回归。

## 4. `AppName` 保留但固定为 `Multica`

两个模板数据结构都保留 `AppName`，因为它让模板作者能复用内置邮件现有品牌词并保持与默认主题一致；
但它不是新增配置项，`{{.AppName}}` 永远渲染为 `Multica`。若部署方要用自有品牌，应直接在模板里写
字面量。把 `AppName` 变成可配置值会违反本次字段准入规则：模板数据只能包含能从同一封 BASE 邮件
已有 Subject/HTML 字节确定性恢复的值。

四语言文档已明确写出这个取舍。验证码 Subject 模板虽然可以引用 `{{.Code}}`，文档也明确警告不要
这样做，因为 Subject 会进入邮件服务器日志与手机锁屏预览。

## 5. Q5a / Q5b 的放行理由

- Q5a：采用 998 octet 上限。它直接对应 RFC 5322 行上限，使自定义路径不再生成新增长行；真实 MTA
  对超长行会拒绝、改写还是放行仍属 `UNVERIFIED`，所以这是按标准收敛风险，不是假装已测过真实 MTA。
- Q5b：超限时按 rune 从尾部截断。这样不会切坏 UTF-8，多次调用幂等，并保留能落在上限内的最长
  rune 前缀。它不如 folding 保真，但不触碰共享内置路径，符合本 PR 的冻结范围。

