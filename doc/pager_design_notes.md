# 游标分页 设计探索 / 踩过的坑

本文记录把一个 "全部聊天室" 列表从 "有 bug 的 cursor 分页" 改造成 "协议稳定可复用的
cursor 分页" 过程中的判断和踩坑. 配套契约见 [pager_design.md](./pager_design.md);
测试矩阵见 `hgmCursorPagerTest/pager_test.go`.

## 一、原始 bug

用户反馈:
* 进入 "全部聊天室" 列表.
* 点一次 "下一页" → 第 2 页, 正常.
* 连点 2 次 "上一页" → 列表变空, 下一页按钮也灰了, 只能刷新.

根因:
* 旧后端: prev 分支硬编码 `HasPrev = (len == pageSize)`.
* 第 2 页 click prev: 后端用 page2 第 1 条作 cursor 查更新的, 拿回 page1 全部 10 条,
  `HasPrev` 被设成 true (因为 len == 10).
* 用户再点 prev: 此时 cursor 是 page1 第 1 条 (= 全集最新一条), 查严格更大 → 0 条.
  列表空, hasPrev = false, 但 hasNext 因为 prev 分支强制 true 还在, 而列表已空,
  GoNext 触发 last = undefined → 永远卡住.

## 二、第一版修复 (临时止血)

只动前端, 引入 currentPage 计数:
* hasPrev 由 `currentPage > 1` 派生, 不依赖后端 HasPrev.
* page 1: 强制不带 cursor.
* page 2 → page 1: 直接重置为 "无 cursor 的首页查询".
* page N (N>=3) → page N-1: 走 prev cursor.

提交后发现这版只修了 "显式 bug", 还有一堆边界没处理:
* 末页正好 pageSize 条时, 后端 HasNext 仍是 true (false-positive), 用户再点 next 会拿 0 条.
* prev 拿到 < pageSize 时, 用户在一个 "虚的中间页" 上, 不是真实页码状态.
* 数据被并发删除/添加, 没有 escape hatch (ws 推送之外, 用户没法主动刷).
* 接口里 (CursorTime, CursorId, Direction) 暴露内部结构, 后端将来想换排序键就得动前端.

于是开第二轮, 系统性优化.

## 三、第二轮讨论里挑过的几个细节

1. **"回首页" 按钮 (page > 2 时显示)** — 多次 prev 之后回首页要点很多次, 不爽;
   而且遇到 ghost-page / 0-条 这类异常状态时, 用户需要一个 "无脑修复" 出口.

2. **向前翻不足 pageSize → 重置首页 (二次拉取)** — 若 prev 返回 < pageSize 条,
   说明这些就是 "全部比 cursor 更新的", 概念上等于首页. 选 "二次拉取" 是为了以
   "真正的当前最新首页" 为准, 不让用户停在一个内容已过期的 "近似首页" 上. 考虑过
   "一次拉取直接当首页用" 省一次请求, 但视觉一闪也避不掉, 收益不明显, 放弃.

3. **向后翻 0 条 → 不前进, 弹提示 + 刷当前页** — 末页正好 == pageSize 时 NextToken 仍非空
   (后端无法判别), 用户点了拿 0 条. 不能灰 next (灰了用户就得 F5 丢 page 状态), 不能改 page
   (拿 0 条等于状态过期信号), 应弹 notice + 主动刷当前页. 重要的是 notice 在 refresh 之后
   还得保留 (sticky), 否则 apply 一发就被清掉了.

4. **向后翻 1..9 条 → next 不可用 (NextToken="")** — 后端规则覆盖到了, 前端无需额外处理.

5. **手动刷新当前页** — 与 ws 自动刷新等价但走主动路径. 点击 ↻ 刷当前页 + 清 notice.
   ws 漏推/初始化竞态时是用户的 escape hatch.

6. **不透明 token (取代 CursorTime, CursorId, Direction 三件套)** — 收益最大的一项.
   前端不再 care 内部结构, 后端将来想换排序键 / 加二级索引 / 加业务过滤都不动前端.
   接口收口为 `{ Token }` / `{ Items, PrevToken, NextToken }`. 第 1 页 token = `""` 这个特例
   必须保留: 否则 token 老化后用户回不到 "最新视角".

7. **在途请求 seq 防竞态** — 用户点 next 紧接着 ws 推 → refresh, 两个 in-flight 请求响应顺序
   不定. 无 seq 校验时旧响应可能覆盖新响应. 引入 `++requestSeq`, 响应回来 seq 不一致就丢.
   异步测试 (Issue + Deliver) 故意让响应乱序释放, 验证丢弃逻辑.

8. **空列表文案区分 page=1 vs page>1** — page=1 空 → "整个项目就没数据"; page>1 空 →
   "翻到这页, 数据已变". 不区分会让用户以为 "数据被删光了", 误导.

9. **pager 常驻** — 加了 ↻ 刷新按钮以后, pager 必须无条件常驻, 哪怕单页/无数据也得能手动刷.

## 四、踩过的坑

* **(a) base64 包装** — 一开始把 `token = base64url(json.Marshal(...))`. 走 POST JSON body
  不是 url query, 不存在转义需求; base64 防瞎改不是安全机制; 体积 +33%, Network 面板看不懂.
  立刻去掉, `token = string(json.Marshal(...))`, 直接可读.

* **(b) "末页正好 pageSize" 是固有 false-positive** — 后端 NextToken 只看 `len >= pageSize`,
  不可能判别 "恰好 pageSize 是末页" 和 "还有更多". 让后端简单, 前端在 GoNext 拿 0 条时兜底.

* **(c) sticky notice 被 apply 清掉** — GoNext 0 条分支设了 notice, 然后立刻 fetch refresh,
  refresh 抵达时进 applyResponse 清 notice. 解决办法是把 "应用后保留的 notice 文本" 当参数
  传下去, apply 完再覆一次.

* **(d) 同业务时间字段大批条目的切片** — 复合键 (time DESC, id DESC) 是必须的, 单 time 不能
  切干净. 测试覆盖 12 条同 time 场景, 验证 page1 / page2 无重叠无丢失.

* **(e) "前端两次拉取" vs "一次拉取"** — 向前翻不足 → 重置首页走 GoFirst (两次拉取),
  而不是把 prev 结果 "近似" 当首页 apply. 后者会漏 "期间可能有新条目" 的情况.

* **(f) 测试独立** — backend simulator 不 import 真实业务的 token codec (会拖进 DB / ORM 依赖),
  而是内联一份 "协议参考实现". 真后端跟测试 simulator 之间的差异, 有 22 条 case 兜着.

## 五、形成的可复用结论

把游标分页拆成三层, 每层各管一摊:

* **后端**: Token 编解码 + SQL 三 case + Prev/Next token 计算规则.
* **前端**: fetch (seq 校验) / apply (单点更新) / 高层动作 (GoFirst / Refresh / GoNext / GoPrev).
* **测试**: 内存 backend simulator + 状态机 frontend simulator + 边界矩阵.

下一次新列表要分页, 直接照 [pager_design.md](./pager_design.md) 抄, 测试矩阵直接复用
`pager_test.go` 的 22 条做参考, 改 `mkData` / `mkBackend` 的业务字段就能跑.

如果将来扩展到 3 处以上同款分页还看着别扭, 再考虑抽 helper. 在那之前, 重复几行
SQL / 状态机比早抽错抽象划算.
