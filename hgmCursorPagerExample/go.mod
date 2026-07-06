// 单独 go.mod 让这份示例是一个独立 module, 不进 hgmCursorPager 库本身的依赖图.
//
// 本示例零依赖 (只用 Go 标准库 + hgmCursorPager 本身), 直接可跑: go run .
// 演示的是 "内存 store + net/http + 三 case cursor 分页" 的端到端接线. 真实项目把内存
// store 换成 DB/ORM, HTTP 层换成自己的 RPC 框架即可, hgmCursorPager 这一侧不变.

module example.com/hgmCursorPagerExample

go 1.19

require github.com/hgmGoLib/hgmCursorPager v0.0.0-00010101000000-000000000000

// hgmCursorPager 是兄弟目录, 用 replace 指过去.
replace github.com/hgmGoLib/hgmCursorPager => ../
