package hgmCursorPagerTest

// Frontend_t 模拟前端 (Sidebar.svelte 的 allChats) 状态机.
//
// 同步模式 (默认): 每个动作会立刻发请求 + 立刻拿响应. 多数测试走这个模式.
// 异步模式: Issue* 方法只入队 pending 请求, Deliver(idx) 才把响应应用. 用于 race 测试,
// 可把请求按任意顺序释放, 验证 seq 丢弃逻辑.
type Frontend_t struct {
	backend *Backend_t

	// ===== 可见状态 =====
	Items       []ChatItem_t
	CurrentPage int
	PrevToken   string
	NextToken   string
	Notice      string
	Loading     bool

	// 取得当前页所用的 token (刷新当前页时复用), '' = 第 1 页.
	pageToken string

	// 在途请求序号; 响应回来若不等于当前 seq 则丢弃.
	requestSeq int

	// 异步模式下未释放的请求.
	pending []pendingRequest_t
}

type pendingAction_t string

const (
	pendingAction_first   pendingAction_t = "first"
	pendingAction_next    pendingAction_t = "next"
	pendingAction_prev    pendingAction_t = "prev"
	pendingAction_refresh pendingAction_t = "refresh"
)

type pendingRequest_t struct {
	Seq          int
	Token        string
	Action       pendingAction_t
	TargetPage   int    // next/prev 进入的目标页码; refresh 用当前页; first 固定 1.
	StickyNotice string // 应用响应后保留这条 notice (rollback 后不被清掉).
}

func NewFrontend(b *Backend_t) *Frontend_t {
	return &Frontend_t{backend: b, CurrentPage: 1}
}

// ============== 同步动作 (常用) ==============

func (f *Frontend_t) GoFirst() { f.deliverImmediate(f.IssueGoFirst()) }
func (f *Frontend_t) Refresh() { f.deliverImmediate(f.IssueRefresh()) }
func (f *Frontend_t) GoNext()  { f.deliverImmediate(f.IssueGoNext()) }
func (f *Frontend_t) GoPrev()  { f.deliverImmediate(f.IssueGoPrev()) }

// HasNext / HasPrev 给 UI / 测试断言用.
func (f *Frontend_t) HasNext() bool { return f.NextToken != "" }
func (f *Frontend_t) HasPrev() bool { return f.CurrentPage > 1 }

// ============== 异步: 入队 ==============

// IssueGoFirst / Issue... 返回是否真的发起了请求 (按钮可能 disabled).
// 同步模式调用 deliverImmediate; 异步模式让外部决定何时 Deliver.
func (f *Frontend_t) IssueGoFirst() bool {
	f.requestSeq++
	f.Loading = true
	f.pending = append(f.pending, pendingRequest_t{
		Seq: f.requestSeq, Token: "", Action: pendingAction_first, TargetPage: 1,
	})
	return true
}

func (f *Frontend_t) IssueRefresh() bool {
	f.requestSeq++
	f.Loading = true
	f.pending = append(f.pending, pendingRequest_t{
		Seq: f.requestSeq, Token: f.pageToken, Action: pendingAction_refresh, TargetPage: f.CurrentPage,
	})
	return true
}

func (f *Frontend_t) IssueGoNext() bool {
	if f.NextToken == "" {
		return false
	}
	f.requestSeq++
	f.Loading = true
	f.pending = append(f.pending, pendingRequest_t{
		Seq: f.requestSeq, Token: f.NextToken, Action: pendingAction_next, TargetPage: f.CurrentPage + 1,
	})
	return true
}

func (f *Frontend_t) IssueGoPrev() bool {
	if f.CurrentPage <= 1 {
		return false
	}
	if f.CurrentPage == 2 || f.PrevToken == "" {
		return f.IssueGoFirst()
	}
	f.requestSeq++
	f.Loading = true
	f.pending = append(f.pending, pendingRequest_t{
		Seq: f.requestSeq, Token: f.PrevToken, Action: pendingAction_prev, TargetPage: f.CurrentPage - 1,
	})
	return true
}

// PendingCount 异步模式下查待释放请求数.
func (f *Frontend_t) PendingCount() int { return len(f.pending) }

// Deliver 释放索引为 idx 的 pending 请求 (按发起时的 backend 状态查询并应用响应).
// 等价于 "请求已抵达后端 → 响应抵达前端". 异步模式下用此函数控制响应顺序.
func (f *Frontend_t) Deliver(idx int) {
	if idx < 0 || idx >= len(f.pending) {
		return
	}
	p := f.pending[idx]
	f.pending = append(f.pending[:idx], f.pending[idx+1:]...)
	resp := f.backend.GetAllChatList(p.Token)
	f.handleResponse(p, resp)
}

// deliverImmediate 同步模式: 一直把 pending 队列吃干净 (可能因为 rollback / reset 触发后续请求).
func (f *Frontend_t) deliverImmediate(issued bool) {
	if !issued {
		return
	}
	// 防止意外死循环, 设置上限.
	for guard := 0; guard < 16 && len(f.pending) > 0; guard++ {
		f.Deliver(len(f.pending) - 1)
	}
}

// handleResponse 决定是否应用响应, 以及触发后续动作 (rollback / 重置首页).
func (f *Frontend_t) handleResponse(p pendingRequest_t, resp PagerResp_t) {
	if p.Seq != f.requestSeq {
		// 在途时被新请求超过, 静默丢弃.
		return
	}
	f.Loading = false
	switch p.Action {
	case pendingAction_first:
		f.applyResponse(resp, "", 1)
		if p.StickyNotice != "" {
			f.Notice = p.StickyNotice
		}
	case pendingAction_refresh:
		f.applyResponse(resp, p.Token, p.TargetPage)
		if p.StickyNotice != "" {
			f.Notice = p.StickyNotice
		}
	case pendingAction_next:
		if len(resp.Chats) == 0 {
			// 并发删除场景: 不前进, 留原位 + 提示, 再异步刷一次当前页.
			notice := "已是最新一页, 数据已变化"
			f.Notice = notice
			f.requestSeq++
			f.Loading = true
			f.pending = append(f.pending, pendingRequest_t{
				Seq: f.requestSeq, Token: f.pageToken, Action: pendingAction_refresh, TargetPage: f.CurrentPage,
				StickyNotice: notice,
			})
			return
		}
		f.applyResponse(resp, p.Token, p.TargetPage)
		if p.StickyNotice != "" {
			f.Notice = p.StickyNotice
		}
	case pendingAction_prev:
		if len(resp.Chats) < PageSize {
			// 向前翻不足 pageSize → 重置首页 (第 2 次拉取).
			f.requestSeq++
			f.Loading = true
			f.pending = append(f.pending, pendingRequest_t{
				Seq: f.requestSeq, Token: "", Action: pendingAction_first, TargetPage: 1,
			})
			return
		}
		f.applyResponse(resp, p.Token, p.TargetPage)
	}
}

func (f *Frontend_t) applyResponse(resp PagerResp_t, requestToken string, pageNumber int) {
	f.Items = resp.Chats
	f.CurrentPage = pageNumber
	f.pageToken = requestToken
	f.PrevToken = resp.PrevToken
	f.NextToken = resp.NextToken
	f.Notice = ""
}
