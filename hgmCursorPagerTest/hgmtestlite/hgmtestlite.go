// Package hgmtestlite 是一个零依赖的极简断言库, 只给 hgmCursorPagerTest 用.
//
// 本库开源后不想拖任何外部测试框架进来, 所以把用到的两个断言 (Equal / Ok)
// 自己实现一份. 断言失败直接 panic (Go testing 里 panic 会让当前测试 fail 并打印栈),
// 因此调用处不需要传 *testing.T.
package hgmtestlite

import (
	"fmt"
	"reflect"
	"strings"
)

// Equal 断言 actual 与 expected 深度相等 (支持 slice / struct). 不等则 panic.
func Equal(actual, expected any, msg ...string) {
	if !reflect.DeepEqual(actual, expected) {
		panic(fmt.Sprintf("hgmtestlite.Equal failed: got %#v, want %#v%s", actual, expected, joinMsg(msg)))
	}
}

// Ok 断言 cond 为 true. 否则 panic.
func Ok(cond bool, msg ...string) {
	if !cond {
		panic(fmt.Sprintf("hgmtestlite.Ok failed%s", joinMsg(msg)))
	}
}

func joinMsg(msg []string) string {
	if len(msg) == 0 {
		return ""
	}
	return " — " + strings.Join(msg, " ")
}
