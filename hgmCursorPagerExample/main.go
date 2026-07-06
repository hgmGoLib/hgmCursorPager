package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// main.go: 启动一个 stdlib net/http server, 内存里放一批 Post + 挂 Post 列表 API.
//
// 本示例故意零依赖 (只用标准库 + hgmCursorPager 本身), 直接可跑:
//   go run .
// 然后:
//   POST http://127.0.0.1:5571/api/GetPostList    body: {"AuthorId":"","Token":""}
//   POST http://127.0.0.1:5571/api/GetPostList    body: {"AuthorId":"u_alice","Token":"<上一次 NextToken>"}
//
// 真实项目通常把内存 store 换成 DB + ORM, HTTP 层换成自己的 RPC 框架. 换掉这些不影响
// hgmCursorPager 的用法 — 库对 Web / 存储层没有任何假设.

// Svc 是单例服务对象. 真实项目里 Posts 换成 DB 句柄 + ORM.
var Svc *Service

type Service struct {
	Posts []Post_D
}

func main() {
	Svc = &Service{Posts: seedPosts()}

	// 挂 JSON RPC 路由: POST /api/<方法名>, body 是 JSON 入参, 返回 JSON 出参.
	http.HandleFunc("/api/GetPostList", func(w http.ResponseWriter, r *http.Request) {
		var req GetPostListReq_t
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJson(w, GetPostList(req))
	})

	addr := "127.0.0.1:5571"
	fmt.Println("[hgmCursorPagerExample] listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func writeJson(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// seedPosts 造 25 条 Post 演示分页 (3 页: 10/10/5). CreateAt 严格递减, id 零填充保证
// 字典序与序号一致. 一半属于 u_alice, 一半属于 u_bob, 用来演示 AuthorId 过滤.
func seedPosts() []Post_D {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	authors := []string{"u_alice", "u_bob"}
	posts := make([]Post_D, 0, 25)
	for i := 0; i < 25; i++ {
		posts = append(posts, Post_D{
			Id:       fmt.Sprintf("id%05d", i),
			AuthorId: authors[i%2],
			Title:    fmt.Sprintf("post-%d", i),
			Body:     fmt.Sprintf("body of post %d", i),
			CreateAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	return posts
}
