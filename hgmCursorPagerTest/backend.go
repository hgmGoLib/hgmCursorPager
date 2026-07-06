package hgmCursorPagerTest

import (
	"cmp"
	"sort"
	"time"

	"github.com/hgmGoLib/hgmCursorPager"
)

const PageSize = 10

// ChatItem_t 模拟一条聊天室记录. 排序键 (LastActivityAt DESC, Id DESC), 是 2 维 cursor 的典型形态.
type ChatItem_t struct {
	Id             string
	Title          string
	LastActivityAt time.Time
}

// PagerResp_t 模拟 GetAllChatList 的响应.
type PagerResp_t struct {
	Chats     []ChatItem_t
	PrevToken string
	NextToken string
}

// chatCursor_t 后端使用的 2 维 cursor: (LastActivityAt DESC, Id DESC).
type chatCursor_t struct {
	LastActivityAt time.Time
	Id             string
}

// Backend_t 后端模拟器. 内存里持有全部条目, 每次查询按 (LastActivityAt DESC, Id DESC) 排序后切片.
type Backend_t struct {
	all []ChatItem_t
}

func NewBackend() *Backend_t {
	return &Backend_t{}
}

func (b *Backend_t) Insert(c ChatItem_t) {
	b.all = append(b.all, c)
}

func (b *Backend_t) Delete(id string) bool {
	for i, c := range b.all {
		if c.Id == id {
			b.all = append(b.all[:i], b.all[i+1:]...)
			return true
		}
	}
	return false
}

func (b *Backend_t) Count() int { return len(b.all) }

// sortedDesc 返回按 (LastActivityAt DESC, Id DESC) 排好序的副本.
func (b *Backend_t) sortedDesc() []ChatItem_t {
	out := make([]ChatItem_t, len(b.all))
	copy(out, b.all)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].LastActivityAt.Equal(out[j].LastActivityAt) {
			return out[i].LastActivityAt.After(out[j].LastActivityAt)
		}
		return out[i].Id > out[j].Id
	})
	return out
}

// GetAllChatList 模拟一个 "全部聊天室" 列表端点 (不含 synthetic / draft 合成项).
// 直接调 hgmCursorPager.PaginateInMemory, 把库本身用作示范.
func (b *Backend_t) GetAllChatList(reqToken string) PagerResp_t {
	page, prevToken, nextToken := hgmCursorPager.PaginateInMemory(
		b.sortedDesc(),
		reqToken,
		PageSize,
		func(c ChatItem_t) chatCursor_t {
			return chatCursor_t{LastActivityAt: c.LastActivityAt, Id: c.Id}
		},
		func(a, b chatCursor_t) int {
			if c := hgmCursorPager.CompareTime(a.LastActivityAt, b.LastActivityAt); c != 0 {
				return c
			}
			return cmp.Compare(a.Id, b.Id)
		},
	)
	return PagerResp_t{Chats: page, PrevToken: prevToken, NextToken: nextToken}
}
