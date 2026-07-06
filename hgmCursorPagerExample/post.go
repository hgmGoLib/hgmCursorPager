package main

import (
	"sort"
	"strings"
	"time"

	"github.com/hgmGoLib/hgmCursorPager"
)

// post.go: "Post 列表 + 按作者过滤" 的完整示范.
// 协议契约: ../doc/pager_design.md + ../README.md.
//
// 为了让这份示例零依赖可编译, 后端用内存 slice 当数据源. 但三 case 的结构 (Decode →
// next/prev/default 三分支 → BuildTokens) 与真实 DB 版**一模一样**, 每个分支旁边都标注了
// 它对应的 SQL. 真实项目把内存 filter 换成 ORM 链式 Where / 手写 SQL 即可, 库这一侧不变.
//
// 提示: 如果你的数据 "天然就在内存里" (小集合), 可以直接用 hgmCursorPager.PaginateInMemory
// 一行搞定, 不必像这里手写三分支. 这里手写是为了对照 DB 版的调用形态.

// ========== 1. 数据模型 ==========
//
// 真实项目里这通常是一行 DB / ORM 记录 (习惯上名字以 _D 结尾). 这里退化成普通 struct.
type Post_D struct {
	Id       string
	AuthorId string
	Title    string
	Body     string
	CreateAt time.Time
}

// ========== 2. 列表页大小常量 ==========
//
// 按 ../doc/pager_design.md 第五章, "前后端共享同一个常量, 手动同步".
// 前端见 front/PostListPage.svelte 的 POST_PAGE_SIZE.
const PostListPageSize = 10

// ========== 3. cursor 类型 ==========
//
// 排序键是 (CreateAt DESC, Id DESC), 所以 cursor 是 2 维的:
//   - CreateAt 是业务时间字段
//   - Id 是 tiebreaker (CreateAt 相同的两条 Post 用 Id 切干净)
//
// 字段名要按大驼峰 ASCII 一致 (CreateAt 不写成 createAt), 与排序键一一对应.
type postListCursor_t struct {
	CreateAt time.Time
	Id       string
}

// cmpCursor 比较两个 cursor 在 (CreateAt DESC, Id DESC) 列表中的位置.
// > 0: a 比 b 更 "新" (在 DESC 列表里更靠前); < 0: 更 "旧"; 0: 相等.
func cmpCursor(a, b postListCursor_t) int {
	if c := hgmCursorPager.CompareTime(a.CreateAt, b.CreateAt); c != 0 {
		return c
	}
	return strings.Compare(a.Id, b.Id)
}

func cursorOf(p Post_D) postListCursor_t {
	return postListCursor_t{CreateAt: p.CreateAt, Id: p.Id}
}

// postListItem_t 是 GetPostList 响应里的一条. 与数据行 Post_D 解耦, 这样就可以加
// AuthorName 之类的附加字段, 也可以隐藏不想暴露的字段 (例如 Body 全文).
type postListItem_t struct {
	Id       string
	AuthorId string
	Title    string
	CreateAt time.Time
}

// GetPostListReq_t / GetPostListResp_t: 接口的 JSON 入参 / 出参.
type GetPostListReq_t struct {
	AuthorId string // 空 = 全部作者; 非空 = 仅该作者. 这是 "附加过滤条件".
	Token    string // 不透明 cursor token. "" = 第 1 页.
}

type GetPostListResp_t struct {
	Posts     []postListItem_t
	PrevToken string
	NextToken string
}

// GetPostList 按作者过滤 + 游标翻页. main.go 把它挂到 POST /api/GetPostList.
//
// 协议:
//   入参: { AuthorId string, Token string }
//   出参: { Posts [], PrevToken string, NextToken string }
func GetPostList(req GetPostListReq_t) GetPostListResp_t {
	// ===== Step 1: 附加过滤条件 =====
	//
	// AuthorId 这种 "业务过滤" 在真实 DB 版里必须放到 SQL 层 (WHERE AuthorId=?), 不要
	// select 全表再内存里过滤 — 单表数据涨到几十万就 OOM 了. 见 ../README.md
	// "业务过滤强烈建议放到 SQL 里" 一节. 这里内存版就直接 filter 演示.
	var filtered []Post_D
	for _, p := range Svc.Posts {
		if req.AuthorId == "" || p.AuthorId == req.AuthorId {
			filtered = append(filtered, p)
		}
	}

	// ===== Step 2: 解 token =====
	//
	// hgmCursorPager.Decode 的类型参数必须显式指定 (返回值才用到 C).
	// 解码失败 / 空 token / Direction 非法 一律返回 hasToken=false, 走第 1 页.
	tok, hasToken := hgmCursorPager.Decode[postListCursor_t](req.Token)

	// ===== Step 3: 三 case "查询" =====
	//
	// 每个分支旁边是它对应的 DB SQL. 真实项目把内存 filter+sort 换成那条 SQL 即可.
	var posts []Post_D
	switch {
	case hasToken && tok.Direction == hgmCursorPager.Direction_prev:
		// SQL: WHERE CreateAt > ? OR (CreateAt = ? AND Id > ?)
		//      ORDER BY CreateAt ASC, Id ASC LIMIT N   然后内存 reverse 回 DESC.
		var cand []Post_D
		for _, p := range filtered {
			if cmpCursor(cursorOf(p), tok.Cursor) > 0 { // 严格更 "新"
				cand = append(cand, p)
			}
		}
		sortAsc(cand)
		if len(cand) > PostListPageSize {
			cand = cand[len(cand)-PostListPageSize:] // ASC 里取最后 N 条 = 最靠近 cursor 的一页
		}
		posts = cand
		hgmCursorPager.ReverseSlice(posts) // reverse 回 DESC, 协议规定 page 永远 DESC

	case hasToken && tok.Direction == hgmCursorPager.Direction_next:
		// SQL: WHERE CreateAt < ? OR (CreateAt = ? AND Id < ?)
		//      ORDER BY CreateAt DESC, Id DESC LIMIT N
		var cand []Post_D
		for _, p := range filtered {
			if cmpCursor(cursorOf(p), tok.Cursor) < 0 { // 严格更 "旧"
				cand = append(cand, p)
			}
		}
		sortDesc(cand)
		if len(cand) > PostListPageSize {
			cand = cand[:PostListPageSize]
		}
		posts = cand

	default:
		// 第 1 页 / token 解码失败 一律走这里.
		// SQL: ORDER BY CreateAt DESC, Id DESC LIMIT N
		cand := append([]Post_D(nil), filtered...)
		sortDesc(cand)
		if len(cand) > PostListPageSize {
			cand = cand[:PostListPageSize]
		}
		posts = cand
	}

	// ===== Step 4: 数据行 → API 响应行 =====
	//
	// 拼附加字段 (例如 AuthorName) 在这里做. 本示例没有附加字段, 直接 1:1 拷贝.
	resp := GetPostListResp_t{Posts: make([]postListItem_t, 0, len(posts))}
	for _, p := range posts {
		resp.Posts = append(resp.Posts, postListItem_t{
			Id:       p.Id,
			AuthorId: p.AuthorId,
			Title:    p.Title,
			CreateAt: p.CreateAt,
		})
	}

	// ===== Step 5: 算 PrevToken / NextToken =====
	//
	// items 必须传 "已 DESC 排好序的 + 不含合成项" 的那一份 (这里就是 posts).
	// 如果业务做了二次过滤剔除部分行, 仍要传 "剔除前" 的, 否则游标会停在被剔除的位置.
	resp.PrevToken, resp.NextToken = hgmCursorPager.BuildTokens(
		posts,
		hasToken,
		tok.Direction,
		PostListPageSize,
		cursorOf,
	)
	return resp
}

// sortDesc / sortAsc 按 (CreateAt, Id) 排序. 真实 DB 版由 SQL 的 ORDER BY 负责, 这里内存排.
func sortDesc(items []Post_D) {
	sort.SliceStable(items, func(i, j int) bool {
		return cmpCursor(cursorOf(items[i]), cursorOf(items[j])) > 0
	})
}

func sortAsc(items []Post_D) {
	sort.SliceStable(items, func(i, j int) bool {
		return cmpCursor(cursorOf(items[i]), cursorOf(items[j])) < 0
	})
}
