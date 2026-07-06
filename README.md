# hgmCursorPager — 游标分页 (cursor pager) 协议核心库

```
go get github.com/hgmGoLib/hgmCursorPager
```

零依赖 (仅 Go 标准库). 一份稳定、可复用、前后端对齐的游标分页协议实现.

适用场景: 列表展示 + 用户翻页. 不允许统计总数, 列表内部无停顿条件.
不适用: 评论树之类的非线性结构.

## 如果你只想动手, 不想读整套文档

1. 先去看 `hgmCursorPagerExample/`(readme.txt + post.go + front/PostListPage.svelte),
   30 分钟把后端 + 前端跑出脑内 model.
2. 再回头读本 README 当 API reference / 边界场景速查.
3. 协议契约本身见 [doc/pager_design.md](./doc/pager_design.md) (本库严格实现这一份),
   设计取舍 / 踩坑记录见 [doc/pager_design_notes.md](./doc/pager_design_notes.md).

## 目录

* [doc/pager_design.md](./doc/pager_design.md) — 协议契约原文 (排序键 / 三 case SQL /
  前端动作分支 / UI 元素 / 测试矩阵). 本库严格按这份实现.
* [doc/pager_design_notes.md](./doc/pager_design_notes.md) — 设计探索 / 踩过的坑.
* `hgmCursorPagerExample/` — 端到端范本 (Web server + ORM + svelte). 配 readme.txt 逐步讲解.
* `hgmCursorPagerTest/` — 22 个 case 的协议测试 + 内存版 backend / frontend 状态机.
  兼作 "协议示范代码", 零依赖可直接 `go test`.

## 核心数据结构

```go
Direction_t                  // 方向枚举: "next" / "prev". 前端不感知.
const Direction_next         // "next"
const Direction_prev         // "prev"

Token_t[C any]               // 不透明 token 内部结构. C 是调用方定义的 cursor struct.
                             // {Direction Direction_t, Cursor C}
```

## 主要 API

**编解码:**

```go
Encode[C any](direction Direction_t, cursor C) string
    // 把 (direction, cursor) JSON 序列化成字符串. 编码失败返回 "".
Decode[C any](s string) (tok Token_t[C], hasToken bool)
    // 解码 token. 空 / JSON 失败 / Direction 非法 一律返回 hasToken=false (按第 1 页处理).
```

**计算 PrevToken / NextToken (DB 版调用者用):**

```go
BuildTokens[T any, C any](
    items []T,             // 必须已按 (cursor 字段 DESC) 排好序, 且不含合成项.
    hasToken bool,
    reqDirection Direction_t,
    pageSize int,
    getCursor func(T) C,
) (prevToken, nextToken string)
```

规则:
* `len(items) == 0`: PrevToken="" / NextToken="".
* `len(items) >= pageSize`: NextToken 编码 last; 否则 NextToken="".
* `!hasToken` (第 1 页): PrevToken="".
* `reqDirection==prev` 且 `len < pageSize` (已到顶): PrevToken="".
* 其余: PrevToken 编码 first.

**内存版三分支切片 (仅 "条目天然就在内存里" 才用, 大表必走 DB 版):**

```go
PaginateInMemory[T any, C any](
    items []T,                        // 已按 DESC 排好
    token string,
    pageSize int,
    getCursor func(T) C,
    cursorCmp func(a, b C) int,       // > 0: a 在 DESC 列表中比 b 更靠前 (更 "新")
) (page []T, prevToken, nextToken string)
```

内部走和 [doc/pager_design.md](./doc/pager_design.md) 一致的三 case.

**辅助:**

```go
ReverseSlice[T any](items []T)   // DB 版 prev 分支 ASC 取出后必须 reverse 回 DESC.
CompareTime(a, b time.Time) int  // time.Time 不在 cmp.Ordered 里, 单独提供.
                                 // a.After(b) → +1; a.Before(b) → -1; 同 → 0.
```

其他类型 (int / string / int64 ...) 直接用 std `cmp.Compare`.

## 调用方约定

* 每个列表自定义自己的 cursor struct (1-3 字段, 字段类型可 JSON 序列化).
* DB 查询的 WHERE 三 case 由调用方手写 (ORM 是字段名链式 API, 不能泛型化).
* 业务过滤 / 合成条目 / N+1 优化属于调用方职责, 库不掺合.
* PageSize 由调用方提供常量 (前后端手动同步).

## 业务过滤强烈建议放到 SQL 里 (容易爆内存防御)

* 默认走 DB 版 (BuildTokens + 手写三 case WHERE), 把所有业务过滤 (Status / Provider /
  Keyword / 时间区间) 翻译成 ORM 链式 Where 或手写 SQL, 让 LIMIT 在数据库侧生效.
* 不要 "先 Select 全表, 然后内存里 sort + 切片". 单表数据涨到几十万时直接 OOM.
* `PaginateInMemory` 仅适用于 "条目天然就在内存里" (例如运行时缓存 / 已聚合好的小集合).
  即使如此, 也要先确认上游集合大小有上限.
* 如果 ORM 链式 API 表达不出过滤 (例如 GROUP BY + HAVING, 或多源 OR 合并), 直接走
  raw SQL, 但不要为了避开 SQL 而退回到内存切片.
* 写 raw SQL 时, 字段值用占位符 `?` 传入, 不要字符串拼接. SQL 里直接出现的标识符
  (表名/列名) 必须是代码里硬编码的常量, 不能拼用户输入.

## 调用样例

### 2 维 cursor + DB 版 (最常用)

```go
type postListCursor_t struct {
    CreateAt time.Time
    Id       string
}

qb := Svc.Orm.Post_D()
if ctx.Req.AuthorId != "" {
    qb = qb.AuthorId().Where_equal(ctx.Req.AuthorId)   // SQL 层过滤, 别走内存
}

tok, hasToken := hgmCursorPager.Decode[postListCursor_t](ctx.Req.Token)

var posts []Post_D
switch {
case hasToken && tok.Direction == hgmCursorPager.Direction_prev:
    qb = qb.Where_or(func() {
        qb.CreateAt().Where_greater(tok.Cursor.CreateAt)
        qb.Where_and(func() {
            qb.CreateAt().Where_equal(tok.Cursor.CreateAt)
            qb.Id().Where_greater(tok.Cursor.Id)
        })
    })
    posts = qb.CreateAt().OrderByAsc().Id().OrderByAsc().Limit(PageSize).MustSelect_list()
    hgmCursorPager.ReverseSlice(posts)
case hasToken && tok.Direction == hgmCursorPager.Direction_next:
    qb = qb.Where_or(func() {
        qb.CreateAt().Where_less(tok.Cursor.CreateAt)
        qb.Where_and(func() {
            qb.CreateAt().Where_equal(tok.Cursor.CreateAt)
            qb.Id().Where_less(tok.Cursor.Id)
        })
    })
    posts = qb.CreateAt().OrderByDesc().Id().OrderByDesc().Limit(PageSize).MustSelect_list()
default:
    posts = qb.CreateAt().OrderByDesc().Id().OrderByDesc().Limit(PageSize).MustSelect_list()
}

ctx.Resp.PrevToken, ctx.Resp.NextToken = hgmCursorPager.BuildTokens(
    posts, hasToken, tok.Direction, PageSize,
    func(p Post_D) postListCursor_t {
        return postListCursor_t{CreateAt: p.CreateAt, Id: p.Id}
    },
)
```

### 1 维 cursor + raw SQL (例: GROUP BY + HAVING)

```go
type submitsCursor_t struct{ LatestId string }

tok, hasToken := hgmCursorPager.Decode[submitsCursor_t](ctx.Req.Token)
// 标识符硬编码, 业务值走 ? 占位符.
const sqlBase = "SELECT ChatId, MAX(Id) AS LatestId FROM DraftHistory_D" +
    " WHERE Kind=? AND Content!='' GROUP BY ChatId"
switch {
case hasToken && tok.Direction == hgmCursorPager.Direction_prev:
    // ... HAVING MAX(Id)>? ORDER BY LatestId ASC LIMIT ?, 然后 ReverseSlice.
case hasToken && tok.Direction == hgmCursorPager.Direction_next:
    // ... HAVING MAX(Id)<? ORDER BY LatestId DESC LIMIT ?
default:
    // ... ORDER BY LatestId DESC LIMIT ?
}
// 然后用 BuildTokens(groups, hasToken, tok.Direction, PageSize, getCursor) 算 token.
```

### 3 维 cursor (例: (UpdateAt, Scope, ChatId))

```go
type draftCursor_t struct {
    UpdateAt time.Time
    Scope    string
    ChatId   string
}
// 三 case WHERE 三层 OR, 每层多加一个 "前级相等 AND 本级 < / >" 子句.
```

### 内存版 (仅小集合)

```go
page, prev, next := hgmCursorPager.PaginateInMemory(
    sortedDesc, ctx.Req.Token, PageSize,
    func(t T) cursor_t { return cursor_t{...} },
    func(a, b cursor_t) int {
        if c := hgmCursorPager.CompareTime(a.Time, b.Time); c != 0 { return c }
        return cmp.Compare(a.Id, b.Id)
    },
)
```

### 前端 svelte 5 接入 (3 行)

```js
const POST_PAGE_SIZE = 10  // 与后端 PostListPageSize 同步
const pager = createCursorPager({
    fetcher: (token) => postJson('/GetPostList', { AuthorId: authorId, Token: token }),
    extractItems: (data) => data.Posts || [],
    pageSize: POST_PAGE_SIZE,
})
// UI: <PagerControls {pager} /> + each pager.items.
// 过滤条件改 → pager.goFirst().
// ws 推 → pager.refresh(true) (silent).
```

完整的 `createCursorPager` 实现 + 5 个动作的协议分支见
`hgmCursorPagerExample/front/cursorPager.svelte.js`, 以及
[doc/pager_design.md](./doc/pager_design.md) 第三章.

## 注意点

* token 跨列表不可换 (cursor 类型不同). 这是协议要求, 不是 bug.
* token 字段扩展兼容: JSON 缺字段 zero value, 多字段忽略, Direction 非法 fallback 第 1 页.
* 不签 token. 用户无法通过伪造 token 越权 (后端权限过滤独立做).
* 末页正好 == pageSize 是 NextToken false-positive: 后端无法判别 "还有没有", 故意不修,
  让前端在 GoNext 拿 0 条时走 sticky notice + silent refresh 分支.
* 合成条目 (例如新建草稿这种插在第 1 页头部的虚拟项): 仅 `!hasToken` 时追加, 不参与 BuildTokens.
* 业务过滤后剔除部分行不展示, 仍要把 "未剔除前" 的传给 BuildTokens, 否则游标会停在被剔除的
  位置, 下一页会重复重取.

## License

MIT
