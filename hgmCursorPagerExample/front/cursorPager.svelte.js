// cursorPager.svelte.js — 游标分页通用状态封装 (svelte 5 $state runes).
//
// 协议契约: ../../doc/pager_design.md + ../../README.md.
// 这是一份纯前端状态机, 任何接 hgmCursorPager 后端的 svelte 5 项目都可以原样复用.
//
// 用法 (见 PostListPage.svelte):
//   const pager = createCursorPager({
//     fetcher: (token) => postJson('/GetPostList', { Token: token, AuthorId: filter.authorId }),
//     extractItems: (data) => data.Posts || [],
//     pageSize: POST_PAGE_SIZE,
//   })
//   pager.goFirst()           // 主动加载第 1 页 (例如初始化或筛选条件变化)
//   pager.refresh(true)       // ws 推送时刷新当前页, 不动 page (silent)
//   pager.goNext() / pager.goPrev() // 翻页
//   $derived: pager.items / pager.currentPage / pager.hasNext / pager.hasPrev / pager.notice / pager.loading
//
// 关键点:
//   * fetcher 只接受 token, 其他过滤条件 (AuthorId / 关键词) 由调用方在闭包里持有.
//   * 过滤条件变化时, 调用方负责调 goFirst() 拉第 1 页.
//   * 前端永远不解析 token, 收到啥就回传啥.
//   * requestSeq 防 ws 自动刷新与点击竞态: 响应回来如果 seq 已经变了, 静默丢.

export function createCursorPager({ fetcher, extractItems, pageSize }) {
  // ===== 可见状态 (UI 直接读取) =====
  let items = $state([])
  let currentPage = $state(1)
  let prevToken = $state('')
  let nextToken = $state('')
  let notice = $state('')
  let loading = $state(false)

  // 取得当前页所用的 token ('' = 第 1 页), 刷新时复用.
  let pageToken = ''
  // 在途请求序号, 防 ws 自动刷新与点击竞态.
  let requestSeq = 0

  // fetchData: 发请求 + seq 校验. 返回 data 或 null (stale / 失败).
  async function fetchData(token, silent) {
    if (!silent) loading = true
    const seq = ++requestSeq
    try {
      const data = await fetcher(token)
      if (seq !== requestSeq) return null
      loading = false
      return data
    } catch (err) {
      if (seq === requestSeq) loading = false
      console.error('cursorPager fetch failed:', err)
      return null
    }
  }

  function applyResponse(data, requestToken, pageNumber) {
    items = extractItems(data)
    currentPage = pageNumber
    pageToken = requestToken
    prevToken = data.PrevToken || ''
    nextToken = data.NextToken || ''
    notice = ''
  }

  async function goFirst() {
    const data = await fetchData('', false)
    if (data) applyResponse(data, '', 1)
  }

  async function refresh(silent = true) {
    const token = pageToken
    const page = currentPage
    const data = await fetchData(token, silent)
    if (data) applyResponse(data, token, page)
  }

  async function goNext() {
    if (!nextToken) return
    const token = nextToken
    const targetPage = currentPage + 1
    const data = await fetchData(token, false)
    if (!data) return
    if (extractItems(data).length === 0) {
      // 并发删除场景: 不前进, 弹提示, 刷一下当前页. (sticky notice 见下面再覆一次.)
      notice = '已是最新一页, 数据已变化'
      const refreshToken = pageToken
      const refreshPage = currentPage
      const refreshData = await fetchData(refreshToken, true)
      if (refreshData) {
        applyResponse(refreshData, refreshToken, refreshPage)
        notice = '已是最新一页, 数据已变化' // applyResponse 会清掉 notice, 这里再覆.
      }
      return
    }
    applyResponse(data, token, targetPage)
  }

  async function goPrev() {
    if (currentPage <= 1) return
    if (currentPage === 2) {
      // page==2 → page==1: 等价 GoFirst, 避免依赖可能为空的 prevToken.
      await goFirst()
      return
    }
    if (!prevToken) {
      // 后端已暗示到顶 (PrevToken==""), 同样回首页.
      await goFirst()
      return
    }
    const token = prevToken
    const targetPage = currentPage - 1
    const data = await fetchData(token, false)
    if (!data) return
    if (extractItems(data).length < pageSize) {
      // 并发删除导致向前翻不足 pageSize → reset 首页 (二次拉取).
      await goFirst()
      return
    }
    applyResponse(data, token, targetPage)
  }

  return {
    // ===== 可见状态 (getters 触发 svelte 5 响应式订阅) =====
    get items() { return items },
    get currentPage() { return currentPage },
    get prevToken() { return prevToken },
    get nextToken() { return nextToken },
    get hasPrev() { return currentPage > 1 },
    get hasNext() { return nextToken !== '' },
    get notice() { return notice },
    set notice(v) { notice = v },
    get loading() { return loading },

    // ===== 动作 =====
    goFirst,
    refresh,
    goNext,
    goPrev,
  }
}
