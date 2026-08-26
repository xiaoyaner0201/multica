---
ts: 20260826-113307
agent: leader
action: supersede
tree: 01d9e4fd44f3c235fbc9eb25dcab0935567c24c1
verdict: —
---

# Superseding：分支一致性判据写错，改按 upstream ref 判定；并补上唯一正确的 push 形式

HD-53 实现在开工第一步即 `blocked`，原因是我在派单里写死的这条规则：

> 「**分支名：`feature/email-template-customization`** —— intake 已定死，全链唯一。
> 发现自己所在分支与此不一致时**停止工作并报告**。」

实现照此执行、停在原地并上报，**行为完全正确**。错的是这条规则本身。本分录取代该判据。
账本 append-only，旧分录保留原样。

## 复算：`multica repo checkout` 从不给出字面分支名

我在本容器（`56e5adfded6b`）原样重跑了派单里的命令，输出如下——**这是命令原始输出，不是转述**：

```
$ multica repo checkout https://github.com/xiaoyaner0201/multica --ref feature/email-template-customization
/workspaces/140e8421-7099-4023-91fd-4c09406f46a2/56e5adfded6b/workdir/multica
Checked out https://github.com/xiaoyaner0201/multica → /workspaces/.../workdir/multica (branch: agent/agent/56e5adfded6b)
```

`multica repo checkout --help` 自述「Creates a git worktree from the daemon's bare clone cache」。
机制是：**每个容器一个 worktree，分支名恒为 `agent/agent/<runtime-id>`，`--ref` 只决定起点 commit
与 upstream**。实现拿到 `agent/agent/52fb424651d1`，我拿到 `agent/agent/56e5adfded6b`，
差异只是容器 id。字面分支名在这个机制下**不可能**出现在任何 agent 的工作区，
所以我那条判据对全链任何角色都必然为真地触发停机。

`git worktree list` 佐证同一 bare repo 下多容器并存：

```
/workspaces/.repos/.../github.com+xiaoyaner0201+multica.git  (bare)
/workspaces/.../56e5adfded6b/workdir/multica   344e89670 [agent/agent/56e5adfded6b]
/workspaces/.../748e803bd703/workdir/multica   9b4007740 [feature/email-template-customization]
/workspaces/.../82ade6013062/workdir/multica   57eef04aa [feat/hd-48-email-template-override]
/workspaces/.../d8b94142d3b7/workdir/multica   acbbe63c4 [hd-47/smtp-from-mailbox]
```

（`748e803bd703` 是规划阶段的容器，其分支为字面名——即历史上确有非 `agent/agent/*` 的检出形态。
这正是我误以为字面名可复现的来源：**我拿一次观察当成了机制**，未复算就写进派单。）

## 新判据：分支**身份**由 upstream ref + BASE 可达性判定，与本地分支名无关

本地分支名不是身份，`branch.<name>.merge` 才是。本容器实测：

```
$ git config --get branch.agent/agent/56e5adfded6b.remote
origin
$ git config --get branch.agent/agent/56e5adfded6b.merge
refs/heads/feature/email-template-customization
$ git merge-base --is-ancestor f74a71060fcdd45318cae6fb7caf7c0d7e71cdfa HEAD && echo YES
YES
```

即 checkout 已把 upstream 正确绑到冻结分支。**下游应当校验的是这三条**，全部满足即身份一致：

1. `git rev-parse --abbrev-ref --symbolic-full-name @{u}` == `origin/feature/email-template-customization`
2. `git merge-base --is-ancestor f74a71060fcdd45318cae6fb7caf7c0d7e71cdfa HEAD` 成立（BASE 可达）
3. `git status` 工作树 clean

任一不成立才停机上报。**本地分支名 `agent/agent/*` 是正常形态，不构成停机理由。**

## 实现报告的 `git pull` 失败：本容器未复现，但给出无状态命令绕开分歧

实现报「无 tracking information」。本容器 `git pull` 实测成功：

```
$ git pull --ff-only
 .../leader/20260826-113113-supersede-u1.md | 75 ++++++++++++++++++++++
 2 files changed, 76 insertions(+)
```

两个容器的 checkout 行为不一致，成因我未能定位，**不写成已查明**。处置是让下游不依赖
tracking 状态：显式写全 remote 与 ref，`git pull --ff-only origin feature/email-template-customization`。
无论 upstream 是否设置都成立。

## 更严重的一处：派单从未写过 push 形式，而默认 `git push` 会把分录推丢

这一条实现还没走到，但走到必错，**比 pull 那条后果重得多**。本容器 `push.default = current`：

```
$ git push --dry-run
To https://github.com/xiaoyaner0201/multica
 * [new branch]          agent/agent/56e5adfded6b -> agent/agent/56e5adfded6b
```

裸 `git push` 会新建一条**远端垃圾分支** `agent/agent/<runtime-id>`，分录根本不落到冻结分支上。
它会「成功」——退出码 0、有 push 输出——但对下游与我的核对等于没写。我的规则
「未 push 的分录对下游等于不存在」在这里会以最隐蔽的方式命中：**推了，且看起来推成功了，但推错了地方。**

唯一正确形式（实测 dry-run 通过）：

```
git push origin HEAD:refs/heads/feature/email-template-customization
```

派单三张卡片全部只写了「`commit` + `push`」，未写 refspec。本条对**实现与验证**同时生效。

## 连带核对（顺手做掉，结论均为无损）

- 冻结方案 `planner/20260826-110423-plan-r1.md` 实测 sha256
  `e70497a207bf1066bc95fe99fda0d01587b1a78ed8bfb8d48e990692f20267f7`，与派单声明一致。
- `origin/feature/email-template-customization` = `9b4007740`，即上一条 supersede 分录
  **已在远端**。我在本次排查中一度因 remote-tracking ref 陈旧而误判它未 push，复算后更正：无分录丢失。
- 实现 checkout 于 11:29:56，supersede 分录 commit 于 11:31:13——实现的起点 `344e89670`
  天然落后一个 commit。这正是派单「开工第一步 `git pull`」存在的理由，机制工作正常。
- 工作树 clean，`index.md` 头部与表身与实际文件一一对应。

## 不构成 finding，冻结方案不变

本条纯属工作区契约错误，不触及需求、方案或代码。冻结方案 subject tree `37fb17c8…` 不变，
`plan-r1` 无需出 r2，闸门 1 追加的三条要求原样有效，`work_partition` 仍为单一单元 U1。
HD-53 原地恢复，不新建卡片、不换 stage 号——**它从未产出过错误产物，只是没能开工。**

## 流程摩擦（追加，与上一条 supersede 同源）

这是我**第三次**把未复算的状态当作事实写进派单：#7577「已在 BASE 中」、发起人「已在上游问询」、
以及本次「检出后分支名会是字面名」。前两次由下游与发起人纠正，这次由下游停机暴露。
三次同型。

上一条 supersede 我给出的教训是「他方状态要么给复算命令与输出，要么标注未复算」。**它不够。**
那两次错的是*他方*状态，这次错的是*我自己下发的命令会产生什么结果*——我把「我写的命令」
默认当成「我验证过的命令」。补一条更硬的：

> **派单中任何一条要求下游原样执行的命令，我必须先在自己容器里原样跑一遍，
> 把真实输出贴进分录，再写进卡片。** 派单命令是我的产物，不是我的意图。

代价是具体的：一个 HIGH_RISK 卡片停摆一轮，往返两次唤醒。而这条规则的执行成本是
在 intake 时多跑一条 `multica repo checkout` ——我本可以在 intake 时用几十秒排除它。

第二条教训，关于停机判据的写法：

> **停机判据必须落在「可观测且与目标语义等价」的量上。** 分支名是显示层，upstream ref 才是身份。
> 我拿显示层当身份，造出一条对全链必然为真的停机条件——判据本身成了故障源。

## 交接

- 状态：IMPLEMENTING（不变）
- 变更：HD-53 派单正文已按本条改写（分支判据、pull 形式、push refspec），卡片置回 `todo` 并唤醒实现
- 未解决：两容器 checkout 行为差异的成因未定位；已用无状态命令绕开，不阻塞交付
- 需要决策：无
