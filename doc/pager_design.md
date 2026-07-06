# 游标分页 (cursor pager) 协议契约

本库严格实现这份契约. 任何接入 hgmCursorPager 的前后端都按本文实现, 配套测试见
`hgmCursorPagerTest/`.

适用场景: 列表展示, 用户翻页. 不允许统计总数, 列表内部无停顿条件.
不适用: 评论树之类的非线性结构.

## 一、协议契约

* 接口入参:
  * `Token string` — 不透明字符串, `""` = 第 1 页.

* 接口出参:
  * `Items []T` — 本页条目, 已按业务排序键 DESC 排好.
  * `PrevToken string` — `""` 表示无上一页.
  * `NextToken string` — `""` 表示无下一页.

* Token 内部结构 (后端独占, 前端不解析):
  * `Direction` — `"next"` | `"prev"`.
  * `Cursor` — 调用方自定义的 1~3 字段 struct, 与排序键一一对应.
* Token 编码: `json.Marshal` 生成的 string. 不要 base64, 不要任何额外包装.
* 前端只持有 token 字符串, 收到啥, 回传啥, 永不 `JSON.parse`.

* PageSize 固定常量 (示例为 10), 前后端共享同一个常量, 手动同步.

## 二、后端实现

* 排序键: (业务时间字段 DESC, Id DESC). 都加 DESC 是为了保证 (time, id) 复合键唯一切片.
  * 业务时间字段: 比如 LastActivityAt / CreateAt / UpdateAt 看场景挑.
  * Id 必须有, 用作 time 重复时的 tiebreaker.

* 处理流程:
  1. `Decode(req.Token)` → `(tok, hasToken)`. 解码失败 / 空字符串 / direction 不合法 →
     `hasToken = false`, 走第 1 页.
  2. 三种 case 选 SQL:
     * `hasToken && direction == "next"`:
       ```
       WHERE time < cursorTime OR (time == cursorTime AND id < cursorId)
       ORDER BY time DESC, id DESC
       LIMIT pageSize
       ```
     * `hasToken && direction == "prev"`:
       ```
       WHERE time > cursorTime OR (time == cursorTime AND id > cursorId)
       ORDER BY time ASC, id ASC
       LIMIT pageSize
       ```
       然后内存里 reverse 回 DESC (`ReverseSlice`).
     * 默认 (第 1 页):
       ```
       ORDER BY time DESC, id DESC
       LIMIT pageSize
       ```
  3. 业务转换 (附加字段, 比如 AuthorName 之类). cursor 计算用纯 items, 不掺合成条目.
  4. 计算 PrevToken / NextToken (`BuildTokens`, 见下).

* PrevToken 规则:
  * 第 1 页 (`!hasToken`): `""`.
  * `direction == "prev"` 且 `len(items) < pageSize`: `""` (已到顶, 暗示前端 reset 首页).
  * 其余情况: `Encode("prev", items[0].cursor)`.

* NextToken 规则:
  * `len(items) >= pageSize`: `Encode("next", items[last].cursor)`.
  * 否则: `""`.
  * 注意: 末页正好 == pageSize 是 false-positive (后端无法判别). 不修复, 让前端在
    GoNext 拿 0 条时走回滚分支.

* 合成条目 (例如 "新建草稿" 这种插在第 1 页头部的虚拟项):
  * 仅 `!hasToken` 时才追加.
  * 不参与游标计算.

* 业务过滤 (Status / Provider / Keyword / 时间区间) 尽量翻译成 SQL WHERE, 让 LIMIT 在
  数据库侧生效. 不要 "先 Select 全表再内存 sort + 切片", 单表数据涨到几十万会 OOM.
  `PaginateInMemory` 仅适用于 "条目天然就在内存里" 的小集合.

## 三、前端实现

* 状态变量 (持有当前页可见状态):
  * `items`
  * `currentPage` — 从 1 起步
  * `pageToken` — 取得当前页所用的 token, `""` = 第 1 页, refresh 时复用
  * `prevToken` / `nextToken` — 来自上一次响应
  * `notice` — 临时提示文案
  * `loading`
  * `requestSeq` — in-flight 序号, 防 ws 自动刷新与点击竞态

* 派生量:
  * `hasPrev = currentPage > 1` (不依赖后端字段)
  * `hasNext = nextToken !== ""`

* 三层函数:
  1. `fetch(token, silent)`: 发请求 + seq 校验. 返回 data 或 null, 不直接动可见 state.
     ```
     const seq = ++requestSeq
     data = await postJson(...)
     if (seq !== requestSeq) return null   // stale, 静默丢
     return data
     ```
  2. `applyResponse(data, requestToken, pageNumber)`: 单点更新 state; `notice = ""`.
  3. 高层动作: GoFirst / Refresh / GoNext / GoPrev. 每个动作 fetch → 检查响应特征 →
     决定 apply 或走分支动作.

* 高层动作:
  * **GoFirst**: `data = fetch("", false); if (data) apply(data, "", 1)`.
  * **Refresh (silent)**: `token = pageToken; page = currentPage; data = fetch(token, silent);
    if (data) apply(data, token, page)`. 刷新当前页, 不动 page.
  * **GoNext**:
    ```
    if (!nextToken) return
    token = nextToken; targetPage = currentPage + 1
    data = fetch(token, false)
    if (!data) return
    if (data.Items.length === 0):
        // 并发删除场景: 不前进, 弹提示, 刷一下当前页.
        notice = "已是最新一页, 数据已变化"
        const refreshData = fetch(pageToken, true)
        if (refreshData) {
            apply(refreshData, pageToken, currentPage)
            notice = "已是最新一页, 数据已变化"   // 再覆一次, apply 会清掉
        }
        return
    apply(data, token, targetPage)
    ```
  * **GoPrev**:
    ```
    if (currentPage <= 1) return
    if (currentPage === 2) { GoFirst(); return }
    if (!prevToken) { GoFirst(); return }   // 后端已暗示到顶
    token = prevToken; targetPage = currentPage - 1
    data = fetch(token, false)
    if (!data) return
    if (data.Items.length < PAGE_SIZE):
        GoFirst()   // 向前翻不足 pageSize → reset 首页, 二次拉取
        return
    apply(data, token, targetPage)
    ```

* ws 推送回调: 调 `Refresh(silent=true)`, 不动 currentPage.

## 四、UI 组件常驻元素

* 翻页器 (无条件常驻, 哪怕只有 1 页, 因为刷新按钮要在):
  * ↻ 刷新按钮: `disabled = loading` — 一直显示.
  * « 首页按钮: `disabled = loading` — currentPage > 2 时显示.
  * 上一页按钮: `disabled = loading || currentPage <= 1` — 一直显示.
  * "第 X 页" 文本: 显示 currentPage — 一直显示.
  * 下一页按钮: `disabled = loading || !nextToken` — 一直显示.

* 空列表文案:
  * `currentPage == 1`: "暂无 XXX".
  * `currentPage > 1`: "本页已空 (数据可能已变化)".

* notice 区: notice 非空时常驻一行 (与列表分开), apply 后被 GoNext 0 条分支重新覆盖一次.

## 五、共享常量

* 后端 `const xxxPageSize = 10`.
* 前端 `const XXX_PAGE_SIZE = 10`. 两边手动保持一致.

## 六、测试

`hgmCursorPagerTest/` 是参考实现 + 22 个测试 case, 把协议钉死. 新列表上线前应跑一遍同样的
测试矩阵, 至少包括:

* 空数据 / 单条 / 正好 PageSize / PageSize+1 / 多页前后翻.
* 同业务时间字段大批数据 (Id 切片正确, 无重叠无丢失).
* 末页正好 == PageSize: 用户点 next 拿 0 条 → 回滚 + notice.
* 并发删除导致 prev 拿 < pageSize → reset 首页.
* 并发删除导致 next 拿 0 条 → 不前进.
* 并发插入新条目 (比 pageToken 更新) 不影响当前页, refresh 也不应让它挤进当前页.
* Token garbage / empty / direction 非法 → fallback 第 1 页.
* Race: 点 next 后立刻 ws 触发 refresh, next 响应被 seq 丢弃.
* Race: 连点 2 次 next, 第 1 个响应被丢弃.
* 第 2 页连点 2 次上一页 不会变空 / 不卡死 (原始 bug 重现, 见 pager_design_notes.md).

## 七、不做

* 不维护 "总条数". 不算总页数. 不显示 "X / Y 页".
* 不在 URL 里持久化页码 / token. 浏览器返回 / F5 自动回到第 1 页, 这是规范允许的.
* 不签名 token. 用户不可能伪造出超出权限的查询位置 (后端权限过滤独立做).
