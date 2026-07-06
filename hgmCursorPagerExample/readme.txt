hgmCursorPagerExample — 端到端"游标分页"对接示范
====================================================

读这个示例的目的
================
hgmCursorPager (库本身) 的 README.md 只讲 API 形状, 看完还是不知道:
  * 后端一个 GetXxxList 端点里, Decode → 三分支 → BuildTokens 到底怎么串.
  * 前端怎么把 fetcher / extractItems / pageSize 三件套拼到 svelte 5 组件上.
  * 过滤条件改了 / ws 推送来了, 该走哪条路径.
  * 一次"点下一页"在前后端到底跑了多少步.

本目录给一个完整可读的范本回答这些. **零依赖, 直接可跑**:

    cd hgmCursorPagerExample
    go run .
    # 然后 POST http://127.0.0.1:5571/api/GetPostList  body {"AuthorId":"","Token":""}

后端用内存 slice 当数据源, 只依赖标准库 + hgmCursorPager 本身. 三 case 的结构 (Decode →
next/prev/default → BuildTokens) 与真实 DB 版**一模一样**, 每个分支旁边标了对应的 SQL.
真实项目把内存 filter 换成 ORM 链式 Where / 手写 SQL 即可, hgmCursorPager 这一侧不变.

scenario: 一个最简单的 "Post 列表 + 按作者过滤" 列表页. 排序键 (CreateAt DESC, Id DESC),
2 维 cursor. 这是 80% 列表的形态; 1D / 3D 见 ../README.md 调用样例段.

目录结构
========
  hgmCursorPagerExample/
  ├── readme.txt              ← 你正在读这个
  ├── go.mod                  ← 单独的 module, require 已发布的 github.com/hgmGoLib/hgmCursorPager
  ├── main.go                 ← net/http server + 内存 store 初始化 (seed 25 条)
  ├── post.go                 ← Post_D 数据模型 + GetPostList 端点 (核心示范)
  └── front/
      ├── api.js                  ← postJson 封装 (POST + JSON 的最薄 RPC helper)
      ├── cursorPager.svelte.js   ← createCursorPager 状态机 (纯 js, 任意项目可复用)
      ├── PagerControls.svelte    ← 5 颗按钮的统一翻页 UI
      └── PostListPage.svelte     ← 列表页范本: 调 createCursorPager + 摆 PagerControls + 列表 each.

  注: front/ 是 svelte 5 源码, 需要你自己的 vite + svelte 工程才能跑; 这里只作对照范本.
  后端 (main.go / post.go) 则是零依赖可直接 go run.

参考文档对照
============
* ../doc/pager_design.md                 ← 协议契约 (本示例严格遵守)
* ../README.md                           ← 库 API 文档
* ../hgmCursorPagerTest/                  ← 后端协议的内存版可跑测试 (22 case)
* 其他 cursor 形态 (库 README.md "调用样例" 段有内联速查):
    2 维 cursor, DB 三 case + 合成条目
    1 维 / 3 维 cursor, 带过滤
    多附加过滤 (Provider / 时间区间 等)

后端: 一次 GetPostList 的完整流程
==================================

下面跟着代码读一遍 post.go GetPostList 的执行路径.

Step 1 — 路由进来
------------------
  http.HandleFunc("/api/GetPostList", ...)   // main.go

stdlib net/http: 收到 POST /api/GetPostList, body JSON 反序列化进 GetPostListReq_t,
调 GetPostList(req), 返回值 JSON 序列化回去.

真实项目通常有自己的 RPC 框架 (自动按方法名注册路由 + 自动 marshal), 换掉这层不影响
hgmCursorPager 用法.

Step 2 — 业务过滤
------------------
  var filtered []Post_D
  for _, p := range Svc.Posts {
      if req.AuthorId == "" || p.AuthorId == req.AuthorId {
          filtered = append(filtered, p)
      }
  }

* AuthorId 这种 "业务过滤" 在真实 DB 版里必须放到 SQL 层 (WHERE AuthorId=?), 不能
  select 全表再内存 filter — 单表数据涨到几十万会 OOM. 见 ../README.md
  "业务过滤强烈建议放到 SQL 里". 这里内存版直接 filter 演示.

Step 3 — 解 token
------------------
  tok, hasToken := hgmCursorPager.Decode[postListCursor_t](req.Token)

* 类型参数 [postListCursor_t] 必须显式写 (返回值才用到 C, 编译器推不出来).
* 空 token / JSON 失败 / Direction 非法 一律 hasToken=false → 走第 1 页分支.
* postListCursor_t 是 2 维 cursor, 字段 (CreateAt time.Time, Id string) 与排序键一一对应.

Step 4 — 三 case 查询
---------------------
按 ../doc/pager_design.md 第二章硬性规定. post.go 里每个分支旁标了对应的 SQL:

  next:  WHERE time <  cursorTime OR (time = cursorTime AND id <  cursorId)
         ORDER BY time DESC, id DESC LIMIT N
  prev:  WHERE time >  cursorTime OR (time = cursorTime AND id >  cursorId)
         ORDER BY time ASC,  id ASC  LIMIT N
         然后内存里 reverse 回 DESC.
  default (第 1 页 / 解码失败):
         ORDER BY time DESC, id DESC LIMIT N

本示例内存版: next/prev 分支用 cmpCursor 过滤出 "更旧 / 更新" 的条目, sort 后取 N 条;
prev 分支取完调 hgmCursorPager.ReverseSlice 把 ASC 翻回 DESC, 这样 BuildTokens 拿到的
page 永远是 DESC. 这点不能省.

为什么 next 和 prev 都是 "OR (相等 AND id <op>)" (即 cmpCursor 用 (time, id) 复合比较)?
因为 time 字段允许重复 (例如同一秒插入两条), 单看 time 的 < / > 会丢/重 数据. 加上 id 切干净.

提示: 如果数据 "天然就在内存里" (小集合), 可以直接 hgmCursorPager.PaginateInMemory 一行
搞定, 不必手写三分支. 这里手写是为了对照 DB 版的调用形态.

Step 5 — 数据行 → API 响应行
----------------------------
  for _, p := range posts {
      resp.Posts = append(resp.Posts, postListItem_t{...})
  }

实际项目里这一段往往要拼附加字段, 比如:
  - AuthorName: 先批量查作者表 (避免 N+1).
  - 状态显示: 把 Status 映射成展示文本.
本示例没附加字段, 1:1 拷贝.

Step 6 — 算 PrevToken / NextToken
----------------------------------
  resp.PrevToken, resp.NextToken = hgmCursorPager.BuildTokens(
      posts, hasToken, tok.Direction, PostListPageSize, cursorOf,
  )

* 第 1 个参数 posts 必须是 "DESC 排好序的 + 不含合成条目" 的那一份.
  - 如果有合成条目 (例如新建草稿这种插在第 1 页头部的虚拟项), 要先用 posts 算 token,
    然后再 prepend 合成条目到响应里.
  - 如果业务做了二次过滤 (例如剔除某些行不展示), 仍然传 "剔除前" 的, 否则游标会
    停在被剔除的位置, 下一页会重复重取.
* hasToken / Direction 决定 PrevToken 是不是 "" (第 1 页 / 已到顶 都给 "").
* len(posts) >= pageSize 决定 NextToken 是不是 ""; 末页正好 == pageSize 时
  NextToken 还会是非空 (false-positive), 让前端在 GoNext 拿 0 条时走回滚.

前端: 一次"点下一页"的完整流程
================================

PostListPage.svelte 是 UI 入口. 关键 3 行接入:

  const POST_PAGE_SIZE = 10  // 与后端 PostListPageSize 同步
  const pager = createCursorPager({
    fetcher: (token) => postJson('/GetPostList', { AuthorId: authorId, Token: token }),
    extractItems: (data) => data.Posts || [],
    pageSize: POST_PAGE_SIZE,
  })

  // ws / 初始化 / authorId 变化都走 pager.goFirst() 或 pager.refresh(true).

* fetcher: 闭包持有 authorId. hgmCursorPager 库本身完全不知道 AuthorId 是什么,
  它只知道传 token. 任何附加过滤条件都靠调用方在闭包里塞.
* extractItems: 后端 Resp.Posts 字段名各列表不一样, 用这个回调统一抽出 items 数组.
* pageSize: 必须与后端一致, 否则 GoPrev "len < pageSize → reset 首页" 判错.

UI 模板见 PostListPage.svelte. 关注:
  * loading + items.length === 0 双判, 第 1 次拉数据时显示"加载中".
  * 空列表两种文案: 第 1 页 "暂无 Post" / 第 N 页 "本页已空 (数据可能已变化)".
  * 列表用 each 渲染, key={p.Id}.
  * 翻页器永远显示 (即使只 1 页, 因为刷新按钮要在), 用 <PagerControls {pager} />.

下面是"用户在第 2 页点下一页"的完整时序:
  1. PagerControls 里 [下一页] 按钮 disabled 检查 (loading || !pager.hasNext)
     都过, 触发 pager.goNext().
  2. cursorPager.svelte.js goNext():
       token = nextToken (= 上次响应的 NextToken)
       const seq = ++requestSeq  // 在途序号增 1
       loading = true
       data = await fetcher(token)  // postJson POST /api/GetPostList
       if (seq !== requestSeq) return null  // 中途有更新的请求超过, 静默丢
       loading = false
  3. 后端 post.go GetPostList 走 next 分支三 case, 返回 {Posts, PrevToken, NextToken}.
  4. 前端 goNext() 检查:
       - 如果 data.Posts.length === 0 (并发删除场景): 不前进, notice = '已是最新一页,
         数据已变化', 触发 silent refresh 当前页, 然后再覆一次 sticky notice (因为
         applyResponse 会清掉 notice).
       - 否则: applyResponse(data, token, currentPage+1). items / currentPage / pageToken
         / prevToken / nextToken 一次性更新, notice 清空.
  5. svelte 5 $state runes 触发 UI 重渲染. 用户看到第 3 页内容.

类似的 "GoPrev / Refresh / GoFirst" 时序见 cursorPager.svelte.js 内的注释.

边界场景速查表
==============

  *  第 1 次进页: $effect 触发 goFirst() → currentPage=1, 无 prevToken.
  *  authorId 改: $effect 触发 goFirst() → 重置回第 1 页, items 换成新过滤.
  *  ws 推送(本示例没用): subscribePostListWs(() => pager.refresh(true))
                          只刷当前页, 不动 page.
  *  末页正好 == pageSize: NextToken 还非空 (false-positive). 用户点 next 拿 0 条
                          → goNext() 不前进 + notice + silent refresh. UI 不会卡死.
  *  第 2 页点上一页: page==2 → 走 GoFirst 特例 (不走 prevToken).
  *  在第 N 页点上一页时数据被删剩很少: 拿到 < pageSize → reset 首页 (goFirst).
  *  快速连点 / ws 与点击竞态: requestSeq 兜底, 旧响应被丢弃.
  *  Token 被前端不小心改了 / 被 garbage 替换: 后端 Decode 返回 hasToken=false →
                          fallback 第 1 页, 不报错.

如何把这套搬到自己的项目
========================

后端:
  1. go get github.com/hgmGoLib/hgmCursorPager.
  2. 定义你的 cursor struct (1 ~ 3 字段, 与排序键对齐).
  3. 写 GetXxxList 端点, 照 post.go 的 6 个 step 抄 (业务过滤 / Decode / 三 case
     查询 / 行→API / BuildTokens / 响应). DB 版把内存 filter 换成 ORM Where / 手写 SQL.
  4. 跑一遍 ../hgmCursorPagerTest 的 22 个测试矩阵 (如果你的列表能用内存版断言,
     直接复用 Backend_t; 否则在你的 DB 上手测覆盖同样矩阵).

前端 (svelte 5):
  1. 把 front/api.js / cursorPager.svelte.js / PagerControls.svelte 直接拷过去.
     (api.js 改一下 fetch 路径前缀.)
  2. 列表页照 PostListPage.svelte 的 3 行接入, fetcher 里塞你自己的过滤参数.
  3. 接上 ws 推送 (有就 refresh(true), 没就略).

非 svelte 5 前端 (react / vue / 原生 dom):
  cursorPager.svelte.js 用了 $state runes 是 svelte 5 特有的, 但状态机本身
  (fetchData / applyResponse / goFirst / goNext / goPrev / refresh) 是纯 js 逻辑,
  替换 $state 为自己的状态库就能用. 关键点是 5 个状态字段 + requestSeq + pageToken,
  以及 5 个动作的协议分支不要改, 否则会和 hgmCursorPager 后端的 token / PrevToken /
  NextToken 规则对不上.
