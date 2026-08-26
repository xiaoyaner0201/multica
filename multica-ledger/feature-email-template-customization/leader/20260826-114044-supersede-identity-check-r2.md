---
ts: 20260826-114044
agent: leader
action: supersede
tree: 01d9e4fd44f3c235fbc9eb25dcab0935567c24c1
verdict: —
---

# Superseding r2：身份校验去掉 `@{u}`，改为只依赖远端 ref 与 commit 图

上一条 supersede（`20260826-113307`）把分支身份判据从「本地分支名」改成「upstream ref」。
实现按新判据执行，**第一条即失败**：

```
$ test "$(git rev-parse --abbrev-ref --symbolic-full-name @{u})" = origin/feature/email-template-customization
fatal: no upstream configured for branch 'agent/agent/52fb424651d1'
```

且 `git config --get branch.agent/agent/52fb424651d1.{merge,remote}` 均 exit 1。
实现再次正确停机、未自行修正工作区。**判据第二次成为故障源，还是我写的。**

## 我上一条分录里那句「本容器实测 checkout 已把 upstream 正确绑好」是错的推广

原文我写的是：

> 「即 checkout 已把 upstream 正确绑到冻结分支。**下游应当校验的是这三条**」

前半句对**我的容器**成立，后半句把它当成了机制。实测证明两个容器的 checkout
在 tracking 上行为不一致：`56e5adfded6b` 有 tracking，`52fb424651d1` 没有。
成因仍未定位——**仍然不写成已查明**。

## 复算：本容器主动复现无 tracking 状态，验证新判据

这次不再拿「我的容器能跑」当证据。我把自己的 tracking 显式删掉，复现实现的环境后再测。
以下为原始命令与原始输出：

```
$ git config --unset branch.agent/agent/56e5adfded6b.remote
$ git config --unset branch.agent/agent/56e5adfded6b.merge
$ git rev-parse --abbrev-ref --symbolic-full-name @{u}
fatal: no upstream configured for branch 'agent/agent/56e5adfded6b'
```

实现的失败在本容器复现成功。随后在**同一无 tracking 状态**下测新判据：

```
$ REMOTE=$(git ls-remote origin refs/heads/feature/email-template-customization | cut -f1)
$ LOCAL=$(git rev-parse HEAD)
remote=f58742809046b72162b437d6fd5819ccc1d1cabb
local =f58742809046b72162b437d6fd5819ccc1d1cabb
CHECK1 PASS
$ git merge-base --is-ancestor f74a71060fcdd45318cae6fb7caf7c0d7e71cdfa HEAD && echo "CHECK2 PASS"
CHECK2 PASS
$ test -z "$(git status --porcelain)" && echo "CHECK3 PASS"
CHECK3 PASS
```

pull 与 push 在无 tracking 下同样成立：

```
$ git pull --ff-only origin feature/email-template-customization
 * branch                feature/email-template-customization -> FETCH_HEAD
Already up to date.

$ git push --dry-run origin HEAD:refs/heads/feature/email-template-customization
Everything up-to-date

$ git push --dry-run          # 裸 push，无 tracking 下仍是同一个陷阱
 * [new branch]          agent/agent/56e5adfded6b -> agent/agent/56e5adfded6b
```

裸 `git push` 建垃圾远端分支这一条与 tracking 无关，上一条分录的警告继续有效。
测毕已将本容器 tracking 复原。

## 新判据（r2）：只用远端 ref 与 commit 图，不读任何本地 config

```
git pull --ff-only origin feature/email-template-customization
test "$(git rev-parse HEAD)" = "$(git ls-remote origin refs/heads/feature/email-template-customization | cut -f1)"
git merge-base --is-ancestor f74a71060fcdd45318cae6fb7caf7c0d7e71cdfa HEAD
test -z "$(git status --porcelain)"
```

三条判据的取值全部来自**远端 ref 与本地 commit 图**——这些量在所有容器里同一，
不受 checkout 的 config 差异影响。`@{u}`、`branch.*.remote`、`branch.*.merge`、
本地分支名一律**不再作为判据**，它们是容器局部状态，不是分支身份。

实现本次上报的三项事实（HEAD = `f58742809`、BASE 可达、工作树 clean）**已满足全部 r2 判据**，
其工作区从一开始就是对的，两次停机都只卡在我的判据上。

## 不构成 finding，冻结方案不变

冻结方案 subject tree `37fb17c8…` 不变，`plan-r1` 无 r2，闸门 1 追加的三条要求原样有效，
`work_partition` 仍为单一单元 U1。HD-53 原地恢复，不新建卡片、不换 stage 号。

## 流程摩擦：同一错误的第四次，且上一条教训被证明不够

前三次（#7577「已在 BASE 中」、发起人「已在上游问询」、「检出后分支名是字面名」）
我归因为「未复算他方状态」，并立了规则：

> 「派单中任何一条要求下游原样执行的命令，我必须先在自己容器里原样跑一遍。」

**这条规则我这次照做了，然后照样错。** 我确实在本容器跑过 `git config --get branch.*.merge`
并贴了真实输出——但我的容器有 tracking，实现的没有。**「在我的容器里跑通」证明的是
它在我的容器里跑通，不是它在下游容器里跑通。** 规则本身有个隐含前提（容器同构），
而这个前提恰恰是错的。

真正的教训比上一条更靠上一层：

> **判据只能建立在跨容器不变的量上。** 远端 ref、commit SHA、commit 图的可达关系，
> 在所有容器里同一；本地分支名、`@{u}`、`branch.*` config、工作区路径，都是容器局部状态。
> 拿局部状态当身份，判据就会在别人的容器里失效——**而我永远测不出来，因为我测的是我的容器。**

配套的方法学修正：**当我无法证明下游环境与我同构时，正确动作不是「跑一遍看看」，
而是「消除对该环境量的依赖」。** 本次两条判据（分支名 → `@{u}` → 远端 ref）的演进
正是这个方向，第一次改只挪了一格，仍落在容器局部状态上，所以又崩了一次。
第二次改直接挪到远端 ref，才跳出这个类别。

代价累计：HIGH_RISK 卡片停摆两轮、四次往返唤醒、实现两次开工即停。
两次停机实现的处置都完全正确（停机、贴原始输出、不自行修正工作区、把决策交回我）。

## 交接

- 状态：IMPLEMENTING（不变）
- 变更：HD-53 派单身份校验改为 r2 判据，卡片置回 `todo` 并唤醒实现
- 未解决：两容器 checkout 的 tracking 行为差异成因未定位；已通过消除依赖绕开，不阻塞交付
- 需要决策：无
