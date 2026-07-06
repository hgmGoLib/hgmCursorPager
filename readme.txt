hgmCursorPager — 游标分页 (cursor pager) 协议核心库
====================================================

完整文档见同目录 README.md (GitHub 首页渲染的那份).

速查:
* 协议契约:        doc/pager_design.md        (本库严格实现这一份)
* 设计取舍 / 踩坑:  doc/pager_design_notes.md
* 端到端范本:      hgmCursorPagerExample/     (后端 + 前端逐步讲解, 配 readme.txt)
* 协议测试:        hgmCursorPagerTest/        (22 个 case, 零依赖可直接 go test)

零依赖 (仅 Go 标准库). import path: github.com/hgmGoLib/hgmCursorPager
