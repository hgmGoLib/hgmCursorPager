package hgmCursorPagerTest

import (
	"fmt"
	"testing"
	"time"

	"github.com/hgmGoLib/hgmCursorPager"

	hgmTest "github.com/hgmGoLib/hgmCursorPager/hgmCursorPagerTest/hgmtestlite"
)

// ============== 数据生成辅助 ==============

// mkData 生成 n 条数据, LastActivityAt 严格递减 (id "i00" 最新, "iNN" 最旧).
// id 用零填充以保证字典序与序号一致 (Id DESC = 序号小者优先).
func mkData(n int) []ChatItem_t {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	items := make([]ChatItem_t, n)
	for i := 0; i < n; i++ {
		items[i] = ChatItem_t{
			Id:             fmt.Sprintf("id%05d", n-i-1), // 序号小 → id 字符串大 → 排在前
			Title:          fmt.Sprintf("chat-%d", i),
			LastActivityAt: base.Add(time.Duration(n-i) * time.Minute),
		}
	}
	return items
}

func mkBackend(n int) *Backend_t {
	b := NewBackend()
	for _, it := range mkData(n) {
		b.Insert(it)
	}
	return b
}

// titlesOf 返回 items 的标题, 用于对比.
func titlesOf(items []ChatItem_t) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

// ============== 基本场景 ==============

func TestEmpty(t *testing.T) {
	f := NewFrontend(NewBackend())
	f.GoFirst()
	hgmTest.Equal(len(f.Items), 0)
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Ok(!f.HasNext())
	hgmTest.Ok(!f.HasPrev())
	hgmTest.Equal(f.PrevToken, "")
	hgmTest.Equal(f.NextToken, "")
}

func TestExactlyOnePage(t *testing.T) {
	// 10 条 = 满 1 页. 后端无法判别还有没有更多, NextToken 仍非空 (false positive).
	// 前端点 next → 拿到 0 条 → 回滚 + 提示, 不会前进.
	f := NewFrontend(mkBackend(PageSize))
	f.GoFirst()
	hgmTest.Equal(len(f.Items), PageSize)
	hgmTest.Ok(f.HasNext(), "刚好 pageSize 时, 后端 false-positive 给 NextToken")
	hgmTest.Ok(!f.HasPrev())

	beforeTitles := titlesOf(f.Items)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 1, "拿到 0 条不该前进")
	hgmTest.Equal(titlesOf(f.Items), beforeTitles)
	hgmTest.Ok(f.Notice != "")
}

func TestExactlyOnePagePlusOne(t *testing.T) {
	// PageSize+1 = 边界: 第 1 页满 10 条, NextToken 非空; 进第 2 页只有 1 条.
	f := NewFrontend(mkBackend(PageSize + 1))
	f.GoFirst()
	hgmTest.Equal(len(f.Items), PageSize)
	hgmTest.Ok(f.HasNext())
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2)
	hgmTest.Equal(len(f.Items), 1)
	hgmTest.Ok(!f.HasNext(), "尾页不足 pageSize, 不该有下一页")
	// 此时点 prev: 走"小于 pageSize → reset to first"分支.
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(len(f.Items), PageSize)
}

func TestNavigateForwardAndBack(t *testing.T) {
	// 50 条 = 5 页 (10 满页).
	f := NewFrontend(mkBackend(50))
	f.GoFirst()
	hgmTest.Equal(f.CurrentPage, 1)

	expected1 := titlesOf(f.Items) // 记下第 1 页内容

	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2)
	expected2 := titlesOf(f.Items)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 3)
	expected3 := titlesOf(f.Items)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 4)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 5)
	hgmTest.Ok(f.HasNext(), "末页 == pageSize 时, NextToken 还在 (后端 false positive); 用户点 next 才会拿 0 条触发回滚")

	// 一路 prev 回去
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 4)
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 3)
	hgmTest.Equal(titlesOf(f.Items), expected3, "prev 回到的第 3 页内容应与初次 next 时一致")
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 2)
	hgmTest.Equal(titlesOf(f.Items), expected2)
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 1, "page==2 → page==1 走 GoFirst 路径")
	hgmTest.Equal(titlesOf(f.Items), expected1)
}

func TestExactly5PagesGoNextOnLast(t *testing.T) {
	// 末页正好 PageSize 时, NextToken 非空, 用户点"下一页"会拿到 0 条 → 回滚 + 提示.
	f := NewFrontend(mkBackend(50))
	f.GoFirst()
	for i := 0; i < 4; i++ {
		f.GoNext()
	}
	hgmTest.Equal(f.CurrentPage, 5)
	hgmTest.Equal(len(f.Items), PageSize)
	hgmTest.Ok(f.HasNext(), "末页 == pageSize 时, NextToken 仍非空 (后端无法判别)")

	beforePage := f.CurrentPage
	beforeTitles := titlesOf(f.Items)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, beforePage, "拿到 0 条不该前进")
	hgmTest.Equal(titlesOf(f.Items), beforeTitles, "拿到 0 条不该改 items")
	hgmTest.Ok(f.Notice != "", "应有 '数据已变化' 提示")
	// refresh 后 NextToken 还是不空 (本页满) — 用户可以再尝试.
	hgmTest.Ok(f.HasNext())
}

// ============== 同 LastActivityAt 大批数据 ==============

func TestSameLastActivityAt(t *testing.T) {
	// 所有 12 条 LastActivityAt 完全相同, 仅靠 Id 排序区分.
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	b := NewBackend()
	for i := 0; i < 12; i++ {
		b.Insert(ChatItem_t{
			Id:             fmt.Sprintf("id%05d", 11-i), // i=0 → id00011 (字典序最大, 排第 1)
			Title:          fmt.Sprintf("same-%d", i),
			LastActivityAt: t0,
		})
	}
	f := NewFrontend(b)
	f.GoFirst()
	hgmTest.Equal(len(f.Items), PageSize)
	hgmTest.Ok(f.HasNext())
	page1 := titlesOf(f.Items)

	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2)
	hgmTest.Equal(len(f.Items), 2, "12 条 - 10 = 2")
	hgmTest.Ok(!f.HasNext(), "len < pageSize, 后端给 NextToken=''")

	page2 := titlesOf(f.Items)
	// 不应该有重复
	seen := map[string]bool{}
	for _, t := range page1 {
		hgmTest.Ok(!seen[t], "page1 内不该重复")
		seen[t] = true
	}
	for _, t := range page2 {
		hgmTest.Ok(!seen[t], "page2 不该与 page1 重叠 (同 LastActivityAt 时 Id 应该切干净): "+t)
		seen[t] = true
	}
	hgmTest.Equal(len(seen), 12)

	// 回 page1
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(titlesOf(f.Items), page1, "回到 page1 内容应一致")
}

func TestSameLastActivityAtPrevTiebreaker(t *testing.T) {
	// 23 条全部同 LastActivityAt, 排满 3 页 (10/10/3). 从 page3 走 prev 回 page2 时,
	// 后端 prev 分支 ASC 排序里所有候选都同 time, 必走 Id tiebreaker. 验证内容稳定切片无重叠.
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	b := NewBackend()
	for i := 0; i < 23; i++ {
		b.Insert(ChatItem_t{
			Id:             fmt.Sprintf("id%05d", 22-i),
			Title:          fmt.Sprintf("same-%d", i),
			LastActivityAt: t0,
		})
	}
	f := NewFrontend(b)
	f.GoFirst()
	hgmTest.Equal(len(f.Items), PageSize)
	page1 := titlesOf(f.Items)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2)
	hgmTest.Equal(len(f.Items), PageSize)
	page2 := titlesOf(f.Items)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 3)
	hgmTest.Equal(len(f.Items), 3)

	// 从 page3 prev 回 page2 (currentPage==3, 不走 GoFirst 特例)
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 2)
	hgmTest.Equal(titlesOf(f.Items), page2, "同 time prev 切片必须用 Id tiebreaker, 内容应与初次 next 一致")

	// 顺便确认无重叠
	seen := map[string]bool{}
	for _, x := range page1 {
		seen[x] = true
	}
	for _, x := range page2 {
		hgmTest.Ok(!seen[x], "page1/page2 不应重叠: "+x)
	}
}

// ============== 并发删除 ==============

func TestConcurrentDeleteThenPrev(t *testing.T) {
	// 30 条 → 3 页. 进 page3, 然后把比 page3-cursor 更新的 17 条删剩 3 条; GoPrev 拿到 3 条 < pageSize → reset 首页.
	b := mkBackend(30)
	f := NewFrontend(b)
	f.GoFirst()
	f.GoNext()
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 3)

	// 删除最新 17 条 (i=0..16): newest 20 条里只剩 3 条 (i=17,18,19), 加上未动的 page3 (i=20..29) 共 13 条.
	// PrevToken cursor 指向 page3 第 1 条 = i=20. 比它新的现在只剩 3 条.
	for i := 0; i < 17; i++ {
		b.Delete(fmt.Sprintf("id%05d", 30-i-1))
	}
	hgmTest.Equal(b.Count(), 13)

	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 1, "拿到 < pageSize 应触发 reset 首页 (二次拉取)")
	hgmTest.Equal(len(f.Items), PageSize, "剩 13 条, 首页满 10 条")
}

func TestConcurrentDeleteAllNextItems(t *testing.T) {
	// 在 page2 时, 把所有比当前页"更旧"的条目删了 → 点下一页拿到 0 条.
	b := mkBackend(20)
	f := NewFrontend(b)
	f.GoFirst()
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2)
	hgmTest.Equal(len(f.Items), 10)

	// 当前 page2 的最后一条 (序号最大 = id 字符串最小). 删除所有比它更旧 — 但 page2 本身是最旧 10 条, 没有更旧.
	// NextToken 非空 (因为 len == pageSize), 但实际下面没数据.
	hgmTest.Ok(f.HasNext(), "末页满 10 条 NextToken 仍非空")

	// 不动数据, 直接试一下点下一页 → 0 条 → 回滚.
	beforeTitles := titlesOf(f.Items)
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2, "拿到 0 条不该前进")
	hgmTest.Equal(titlesOf(f.Items), beforeTitles)
	hgmTest.Ok(f.Notice != "")
}

// ============== Refresh ==============

func TestRefreshOnFirstPage(t *testing.T) {
	b := mkBackend(15)
	f := NewFrontend(b)
	f.GoFirst()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(len(f.Items), 10)

	// 在第 1 页插入新数据 — 应该被 refresh 反映.
	b.Insert(ChatItem_t{
		Id:             "newchat",
		Title:          "newchat",
		LastActivityAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	f.Refresh()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(f.Items[0].Title, "newchat", "刷新后应在最前")
}

func TestRefreshOnPageN(t *testing.T) {
	b := mkBackend(40)
	f := NewFrontend(b)
	f.GoFirst()
	f.GoNext()
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 3)
	page3Before := titlesOf(f.Items)

	// 不改数据, refresh 应回到完全相同状态.
	f.Refresh()
	hgmTest.Equal(f.CurrentPage, 3)
	hgmTest.Equal(titlesOf(f.Items), page3Before, "refresh 不改数据时应一致")
}

// ============== Token 不透明性 / 异常 ==============

func TestGarbageTokenFallsBackToFirstPage(t *testing.T) {
	b := mkBackend(25)
	resp := b.GetAllChatList("not json {{{")
	// 无法解码 → 当首页处理.
	hgmTest.Equal(len(resp.Chats), 10)
	hgmTest.Ok(resp.NextToken != "")
	hgmTest.Equal(resp.PrevToken, "")
}

func TestEmptyTokenIsFirstPage(t *testing.T) {
	b := mkBackend(15)
	resp := b.GetAllChatList("")
	hgmTest.Equal(len(resp.Chats), 10)
	hgmTest.Equal(resp.PrevToken, "")
	hgmTest.Ok(resp.NextToken != "")
}

// ============== 边界 ==============

func TestSingleItem(t *testing.T) {
	f := NewFrontend(mkBackend(1))
	f.GoFirst()
	hgmTest.Equal(len(f.Items), 1)
	hgmTest.Ok(!f.HasNext())
	hgmTest.Ok(!f.HasPrev())
}

func TestPrevOnFirstPageNoop(t *testing.T) {
	f := NewFrontend(mkBackend(50))
	f.GoFirst()
	hgmTest.Equal(f.CurrentPage, 1)
	f.GoPrev() // 应该 noop
	hgmTest.Equal(f.CurrentPage, 1)
}

func TestNextWhenNoNextNoop(t *testing.T) {
	f := NewFrontend(mkBackend(5))
	f.GoFirst()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Ok(!f.HasNext())
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 1, "NextToken 空时点击应 noop")
}

// ============== Race (异步释放顺序) ==============

func TestRaceClickNextThenRefreshDiscardsStaleNext(t *testing.T) {
	// 模拟: 用户点 next, 紧接着 ws 触发 refresh (在 next 响应抵达前).
	// 期望: refresh 的 seq 更大, next 响应被丢弃.
	b := mkBackend(50)
	f := NewFrontend(b)
	f.GoFirst() // 先到 page 1
	hgmTest.Equal(f.PendingCount(), 0)

	// 用户点 next
	hgmTest.Ok(f.IssueGoNext())
	hgmTest.Equal(f.PendingCount(), 1)
	nextSeq := f.requestSeq

	// ws 触发 refresh
	hgmTest.Ok(f.IssueRefresh())
	hgmTest.Equal(f.PendingCount(), 2)
	refreshSeq := f.requestSeq
	hgmTest.Ok(refreshSeq > nextSeq)

	// next 响应先抵达 — 应被 seq 检查丢弃.
	f.Deliver(0)
	hgmTest.Equal(f.CurrentPage, 1, "next 响应应被丢弃, page 不变")

	// refresh 响应抵达 — 应用; 仍是 page 1.
	f.Deliver(0)
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(len(f.Items), PageSize)
}

func TestRaceTwoNextDiscardsFirst(t *testing.T) {
	// 用户连点 2 次 next, 第 1 个响应抵达时已被第 2 个超过, 应丢弃.
	b := mkBackend(40)
	f := NewFrontend(b)
	f.GoFirst()

	hgmTest.Ok(f.IssueGoNext()) // 想去 page 2
	hgmTest.Ok(f.IssueGoNext()) // 想去 page 3 (但还在 page 1)
	hgmTest.Equal(f.PendingCount(), 2)

	// 注意: 第 2 次入队时, NextToken 还是去 page 2 的那个 (应用之前没换). 所以两次 token 一样,
	// 两次都会拿到 page 2 数据. 但第 1 个会被 seq 丢弃 (期望 page=2 的 targetPage 会失效).
	// 实际期望: 第 1 个被丢弃, 第 2 个 apply, currentPage = 2 (因为 token 是 page 2 的).
	// 但是 targetPage = currentPage+1 = 1+1 = 2, 第 2 次也是 1+1 = 2 (issue 时 currentPage 还是 1).
	// 所以最终 page=2.
	f.Deliver(0)
	hgmTest.Equal(f.CurrentPage, 1, "第 1 个 next 响应应被丢弃")
	f.Deliver(0)
	hgmTest.Equal(f.CurrentPage, 2)
}

// ============== Token 内部结构稳定性 ==============

func TestTokensRoundTrip(t *testing.T) {
	tt := time.Date(2026, 4, 29, 12, 30, 45, 123456789, time.UTC)
	tokStr := hgmCursorPager.Encode(hgmCursorPager.Direction_next, chatCursor_t{LastActivityAt: tt, Id: "id12345"})
	tok, ok := hgmCursorPager.Decode[chatCursor_t](tokStr)
	hgmTest.Ok(ok)
	hgmTest.Equal(tok.Direction, hgmCursorPager.Direction_next)
	hgmTest.Ok(tok.Cursor.LastActivityAt.Equal(tt))
	hgmTest.Equal(tok.Cursor.Id, "id12345")
}

func TestTokenInvalidDirectionRejected(t *testing.T) {
	// 手工构造一个 direction 非法的 token, 应被解码失败 (回退到首页).
	bad := hgmCursorPager.Encode(hgmCursorPager.Direction_t("garbage"), chatCursor_t{LastActivityAt: time.Now(), Id: "x"})
	_, ok := hgmCursorPager.Decode[chatCursor_t](bad)
	hgmTest.Ok(!ok, "非法 direction 应解码失败")
}

// ============== 并发插入 ==============

func TestConcurrentInsertNewerThanCursor(t *testing.T) {
	// 在 page2 时插入新条 (LastActivityAt 极新). Refresh 当前页时不应该把新条挤进来 —
	// 因为 PageToken 是 next 方向, 新条比 cursor 更新, 自然不会进 page2 的查询结果.
	b := mkBackend(20)
	f := NewFrontend(b)
	f.GoFirst()
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2)
	page2Titles := titlesOf(f.Items)

	// 极新条目插入
	b.Insert(ChatItem_t{
		Id:             "newest",
		Title:          "newest",
		LastActivityAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	f.Refresh()
	hgmTest.Equal(f.CurrentPage, 2, "refresh 不动页码")
	hgmTest.Equal(titlesOf(f.Items), page2Titles, "page2 内容不该因极新插入而变 (页位置由 cursor 决定)")

	// GoPrev (走 page=2 的 GoFirst 特例) → 第 1 页应包含新条.
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(f.Items[0].Title, "newest", "首页第 1 条应是最新")
}

// ============== Refresh 不会因为响应里 Notice 而清掉 sticky ==============

func TestNoticeStickyAfterRefresh(t *testing.T) {
	// 触发 GoNext 0 条 rollback 后的 sticky notice; 后续主动 Refresh 应清掉 (用户认为已知, 重新看到内容就好).
	f := NewFrontend(mkBackend(PageSize))
	f.GoFirst()
	f.GoNext() // 触发 rollback + notice
	hgmTest.Ok(f.Notice != "")
	f.Refresh()
	hgmTest.Equal(f.Notice, "", "用户主动 refresh 后 notice 应清掉")
}

// ============== "用户原始 bug 重现": 第 2 页连点 2 次上一页 ==============

func TestRegressionPrevTwiceFromPage2(t *testing.T) {
	// 用户报告的原 bug 场景: 在第 2 页, 连点 2 次上一页, 列表变空 + 下一页灰.
	// 修复后: 第 1 次回首页, 第 2 次按钮 disabled (HasPrev=false), 不会发请求.
	f := NewFrontend(mkBackend(50))
	f.GoFirst()
	f.GoNext()
	hgmTest.Equal(f.CurrentPage, 2)
	hgmTest.Equal(len(f.Items), PageSize)

	// 第 1 次 prev → 回首页.
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(len(f.Items), PageSize, "回首页后应满 10 条, 不是空")
	hgmTest.Ok(f.HasNext(), "首页应仍能下一页")

	// 第 2 次 prev → noop (UI 上按钮也是 disabled).
	hgmTest.Ok(!f.HasPrev())
	f.GoPrev()
	hgmTest.Equal(f.CurrentPage, 1)
	hgmTest.Equal(len(f.Items), PageSize, "noop, 列表不变")
	hgmTest.Ok(f.HasNext())
}
