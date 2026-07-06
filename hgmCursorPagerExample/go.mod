// 单独 go.mod 让这份示例是一个独立 module, 不进 hgmCursorPager 库本身的依赖图.
//
// 本示例除了 hgmCursorPager 本身没有别的依赖 (只用 Go 标准库), 像真实外部使用者一样
// 直接 require 已发布的 hgmCursorPager 版本, go run . 即可.
// 演示的是 "内存 store + net/http + 三 case cursor 分页" 的端到端接线. 真实项目把内存
// store 换成 DB/ORM, HTTP 层换成自己的 RPC 框架即可, hgmCursorPager 这一侧不变.

module example.com/hgmCursorPagerExample

go 1.19

require github.com/hgmGoLib/hgmCursorPager v0.0.1
