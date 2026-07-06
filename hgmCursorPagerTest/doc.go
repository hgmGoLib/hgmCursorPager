// Package hgmCursorPagerTest 用 golang 同时模拟一个游标分页列表的"前端 + 后端",
// 在内存里跑分页协议, 不碰 DB / HTTP. 用来覆盖各种极端 / 边界 / race case,
// 同时给真实业务调用方提供一份可读的"协议示范代码".
//
// 协议契约 (与 hgmCursorPager 顶层 README.md + doc/pager_design.md 对齐):
//   - 入参: { Token string }. Token 不透明; "" = 第 1 页.
//   - 出参: { Items, PrevToken, NextToken string }.
//   - PrevToken == "" 表示无上一页 (含: 第 1 页本身, 或 prev 方向已到顶 len<pageSize).
//   - NextToken == "" 表示无下一页 (len < pageSize).
//   - Token 内部为 hgmCursorPager.Token_t[C] JSON 序列化, 但前端绝不解析.
//
// 前端协议 (Frontend_t):
//   - 持有 currentPage / pageToken / prevToken / nextToken / requestSeq.
//   - GoFirst: token=""; 应用响应到 page=1.
//   - Refresh: 用 pageToken 重发, 不动 page.
//   - GoNext: 用 nextToken; 若返回 0 条则不前进, 弹提示并刷当前页 (并发删除场景).
//   - GoPrev:
//       page<=1: noop.
//       page==2 或 prevToken=="" : 等价 GoFirst.
//       否则: 用 prevToken; 若返回 < pageSize 条, 走 GoFirst (两次拉取).
//   - 在途请求 seq: 响应到达时 seq != 当前值则丢弃 (防 ws 自动刷新与点击竞态).
//
// 后端 (Backend_t) 用的是 hgmCursorPager.PaginateInMemory + 一个 (LastActivityAt DESC, Id DESC)
// 排序的内存集合. 真实业务通常走 DB 版 (BuildTokens + 手写三 case SQL),
// 但行为契约必须与本示范完全一致, 否则前端状态机会出错.
package hgmCursorPagerTest
