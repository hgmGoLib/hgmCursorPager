# 对接 checklist

给一个新列表接入 hgmCursorPager 时逐条过。端到端范本见 `../hgmCursorPagerExample/`，
协议契约见 [pager_design.md](./pager_design.md)。

## 0. 先判断适不适用

- [ ] 列表是**一维线性、单一排序键**的序列 (不是评论树 / 分组折叠 / 中间有停顿分隔)。
- [ ] 交互只需要 "下一页 / 上一页 / 回首页 / 刷新"，**不需要跳到任意页码**。
- [ ] 不需要 "共 X 条 / 第 X / Y 页" 这种总数展示 (为什么见 [design_rationale.md](./design_rationale.md))。

以上任一不满足，hgmCursorPager 不是对的工具。

## 1. 选排序键 + 定义 cursor struct

- [ ] 选定排序键: `(业务时间字段 DESC, Id DESC)`。业务时间字段按场景挑 (CreateAt / UpdateAt /
      LastActivityAt …)，`Id` 作 tiebreaker **必须有**，否则同时间的行切不干净。
- [ ] 定义 cursor struct，字段与排序键**一一对应**，1~3 个字段，类型可 JSON 序列化。
- [ ] 字段名大驼峰、ASCII 一致 (`CreateAt` 不写成 `createAt`)。
- [ ] 排序字段在 DB 上有对应索引 (复合索引 `(业务时间, Id)`)，否则深翻页仍然慢。

## 2. 后端: 常量 + 端点骨架

- [ ] 定义 `const XxxPageSize = 10` (前后端共享，手动同步)。
- [ ] 端点入参含 `Token string`；出参含 `Items []T` / `PrevToken string` / `NextToken string`。
- [ ] 业务过滤条件 (AuthorId / Status / Provider / Keyword / 时间区间) 放在**入参里**，
      并在 SQL 层 `WHERE` 过滤，**不要** select 全表再内存 filter (大表会 OOM)。

## 3. 后端: 三 case 查询 (照 example/post.go 抄)

- [ ] `tok, hasToken := hgmCursorPager.Decode[cursor_t](req.Token)`，类型参数显式写。
- [ ] `hasToken && next`: `WHERE time < ? OR (time = ? AND id < ?)` ORDER BY DESC LIMIT N。
- [ ] `hasToken && prev`: `WHERE time > ? OR (time = ? AND id > ?)` ORDER BY **ASC** LIMIT N，
      取出后 `hgmCursorPager.ReverseSlice(items)` 翻回 DESC (**别漏**)。
- [ ] `default` (第 1 页 / 解码失败): ORDER BY DESC LIMIT N。
- [ ] raw SQL 时业务值用 `?` 占位符；表名/列名只能是代码里硬编码常量，不拼用户输入。

## 4. 后端: 算 token + 合成条目

- [ ] `resp.PrevToken, resp.NextToken = hgmCursorPager.BuildTokens(items, hasToken, tok.Direction, PageSize, getCursor)`。
- [ ] 传给 `BuildTokens` 的 items 是 "**DESC 排好序 + 不含合成条目**" 的那一份。
- [ ] 有合成条目 (如"新建草稿"这种插在第 1 页头部的虚拟项): **先**用纯 items 算 token，
      **再** prepend 合成条目，且合成条目仅 `!hasToken` 时追加。
- [ ] 有二次业务过滤 (剔除部分行不展示): 仍把 "**剔除前**" 的传给 `BuildTokens`，
      否则游标会停在被剔除的位置，下一页重复重取。

## 5. 前端 (svelte 5, 照 example/front 抄)

- [ ] 拷 `api.js` / `cursorPager.svelte.js` / `PagerControls.svelte` (状态机别改)。
- [ ] `const XXX_PAGE_SIZE = 10`，**必须与后端一致** (否则 GoPrev 的 "len < pageSize → 首页" 判错)。
- [ ] `createCursorPager({ fetcher, extractItems, pageSize })`: fetcher 闭包里塞你的过滤参数，
      extractItems 抽出后端响应里的 items 数组。
- [ ] 初始化 + 过滤条件变化 → `pager.goFirst()`。
- [ ] ws 推送 → `pager.refresh(true)` (silent，不动 page)。
- [ ] `<PagerControls {pager} />` 常驻 (哪怕只有 1 页，刷新按钮要在)。
- [ ] 空列表两种文案: 第 1 页 "暂无 XXX" / 第 N 页 "本页已空 (数据可能已变化)"。
- [ ] 非 svelte 项目: 状态机是纯 js，替换 `$state` 为自己的状态库即可，5 个动作的协议分支别改。

## 6. 测试 (照 ../hgmCursorPagerTest 的 22 case 矩阵)

- [ ] 能用内存版断言的: 直接复用 `Backend_t` / `Frontend_t`，改 `mkData` 的业务字段。
- [ ] 否则在你的 DB 上手测，至少覆盖:
  - [ ] 空 / 单条 / 正好 PageSize / PageSize+1 / 多页前后翻。
  - [ ] 同业务时间字段大批数据 (Id 切片正确，无重叠无丢失)。
  - [ ] 末页正好 == PageSize: 点 next 拿 0 条 → 回滚 + notice，不卡死。
  - [ ] 并发删除导致 prev 拿 < pageSize → reset 首页；next 拿 0 条 → 不前进。
  - [ ] 并发插入更新的条目不影响当前页，refresh 也不让它挤进来。
  - [ ] Token garbage / empty / 非法 direction → fallback 第 1 页。
  - [ ] Race: 点 next 后立刻 ws refresh，旧响应被 seq 丢弃；连点 2 次 next，第 1 个被丢弃。

## 7. 上线前 self-check

- [ ] 前后端 PageSize 常量确实相等。
- [ ] prev 分支的 `ReverseSlice` 没漏。
- [ ] 排序字段有索引，深翻页 explain 走索引不是全表扫。
- [ ] 业务过滤都在 SQL 层，没有"先全量拉再内存切"。
