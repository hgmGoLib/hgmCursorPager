package hgmCursorPager

import "time"

// ReverseSlice 原地反转切片. DB 版 prev 分支 ASC 取出后必须 reverse 回 DESC.
func ReverseSlice[T any](items []T) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

// CompareTime 比较两个 time.Time. time.Time 不在 cmp.Ordered 里, std cmp.Compare 比不了.
// a 晚于 b → +1; a 早于 b → -1; 同 → 0.
func CompareTime(a, b time.Time) int {
	if a.After(b) {
		return 1
	}
	if a.Before(b) {
		return -1
	}
	return 0
}
