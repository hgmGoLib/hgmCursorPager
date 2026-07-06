package hgmCursorPager

// PaginateInMemory 内存版三分支切片.
//
//	items:     已按 (cursor DESC) 排好序的全部条目.
//	token:     请求传入的不透明 token.
//	pageSize:  本列表的 PageSize.
//	getCursor: 把 item 转成 cursor 值.
//	cursorCmp: 比较两个 cursor 在 DESC 列表中的位置. > 0 表示 a 比 b 更靠前 (更"新"); < 0 更靠后 (更"旧"); 0 相等.
//
// 协议三分支:
//   - hasToken && next: 找所有 cursorCmp(item.cursor, tokCursor) < 0 的条目, 取前 pageSize 条.
//   - hasToken && prev: 找所有 cursorCmp(item.cursor, tokCursor) > 0 的条目, 取最近 pageSize 条 (保持 DESC).
//   - 其他 (第 1 页 / 解码失败): items[:pageSize].
//
// 适用于"先在内存里排好序, 然后切片"的场景 (例如多张表过滤聚合后再分页).
// DB 版直接走 ORM 三 case SQL 即可, 不要绕这个函数.
func PaginateInMemory[T any, C any](
	items []T,
	token string,
	pageSize int,
	getCursor func(T) C,
	cursorCmp func(a, b C) int,
) (page []T, prevToken, nextToken string) {
	tok, hasToken := Decode[C](token)
	if len(items) == 0 {
		return nil, "", ""
	}

	switch {
	case hasToken && tok.Direction == Direction_next:
		// next: 找第一个严格更"旧" (cursorCmp < 0) 的位置, 取 pageSize 条.
		startIdx := len(items)
		for i, item := range items {
			if cursorCmp(getCursor(item), tok.Cursor) < 0 {
				startIdx = i
				break
			}
		}
		endIdx := startIdx + pageSize
		if endIdx > len(items) {
			endIdx = len(items)
		}
		if startIdx < endIdx {
			page = items[startIdx:endIdx]
		}
	case hasToken && tok.Direction == Direction_prev:
		// prev: 找所有严格更"新" (cursorCmp > 0) 的条目, 取最后 pageSize 条 (维持 DESC).
		endIdx := 0
		for i, item := range items {
			if cursorCmp(getCursor(item), tok.Cursor) <= 0 {
				endIdx = i
				break
			}
			endIdx = i + 1
		}
		startIdx := endIdx - pageSize
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx < endIdx {
			page = items[startIdx:endIdx]
		}
	default:
		// 第 1 页或解码失败.
		if len(items) > pageSize {
			page = items[:pageSize]
		} else {
			page = items
		}
	}

	prevToken, nextToken = BuildTokens(page, hasToken, tok.Direction, pageSize, getCursor)
	return page, prevToken, nextToken
}
