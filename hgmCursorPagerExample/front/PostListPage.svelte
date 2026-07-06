<script>
  // PostListPage.svelte — 一个完整的"按作者过滤 + 游标翻页"列表页范本.
  //
  // 与后端 post.go GetPostList 一一对应:
  //   POST /api/GetPostList   {AuthorId, Token}  →  {Posts, PrevToken, NextToken}
  //
  // 关注以下 3 个细节:
  //   1) POST_PAGE_SIZE 与后端 PostListPageSize 必须手动对齐 (协议规定的 "前后端共享").
  //   2) 过滤条件变化 (authorId 改) → 调 pager.goFirst() 重新拉第 1 页.
  //   3) ws 推送场景 → pager.refresh(true) silent 刷新当前页, 不动 page.

  import { postJson } from './api.js'
  import { createCursorPager } from './cursorPager.svelte.js'
  import PagerControls from './PagerControls.svelte'

  // 后端 const PostListPageSize = 10. 手动同步.
  const POST_PAGE_SIZE = 10

  // 附加过滤条件 (authorId). svelte 5 $state, 改了之后下面 $effect 触发 goFirst.
  let authorId = $state('')

  // 关键: fetcher 闭包持有 authorId, 自己拼到请求里.
  // hgmCursorPager 库本身不知道 "AuthorId" 是什么, 它只看 token.
  const pager = createCursorPager({
    fetcher: (token) => postJson('/GetPostList', { AuthorId: authorId, Token: token }),
    extractItems: (data) => data.Posts || [],
    pageSize: POST_PAGE_SIZE,
  })

  // 初次加载.
  $effect(() => {
    pager.goFirst()
  })

  // authorId 变化 → 重拉第 1 页. 不手动重置, goFirst 会把 currentPage 设为 1.
  // (svelte 5 $effect 依赖追踪会自动 detect authorId 被读取.)
  $effect(() => {
    // 显式读一下 authorId, 让 effect 订阅它的变化.
    const _ = authorId
    pager.goFirst()
  })

  // 真实项目里, ws 推送可以这样接 (例: 收到 post 创建/删除消息时):
  //   $effect(() => {
  //     const leave = subscribePostListWs(() => { pager.refresh(true) })
  //     return leave
  //   })
  // 这里没有 ws, 略.

  function fmtTime(t) {
    if (!t) return ''
    const d = new Date(t)
    return d.toLocaleString()
  }
</script>

<div class="page">
  <h2>Post 列表</h2>

  <div class="filter-row">
    作者过滤 (留空 = 全部):
    <input bind:value={authorId} placeholder="例: u_alice" />
  </div>

  {#if pager.loading && pager.items.length === 0}
    <div class="hint">加载中...</div>
  {:else if pager.items.length === 0}
    <div class="hint">
      {pager.currentPage > 1 ? '本页已空 (数据可能已变化)' : '暂无 Post'}
    </div>
  {:else}
    <ul class="post-list">
      {#each pager.items as p (p.Id)}
        <li>
          <span class="post-title">{p.Title || '(无标题)'}</span>
          <span class="post-meta">作者 {p.AuthorId} · {fmtTime(p.CreateAt)}</span>
        </li>
      {/each}
    </ul>
  {/if}

  <div class="pager-wrap">
    <PagerControls {pager} />
  </div>
</div>

<style>
  .page { max-width: 720px; margin: 16px auto; font: 14px sans-serif; }
  .filter-row { margin: 8px 0; display: flex; gap: 8px; align-items: center; }
  .filter-row input { padding: 4px 6px; border: 1px solid #ccc; border-radius: 4px; }
  .hint { padding: 16px; color: #999; text-align: center; }
  .post-list { list-style: none; padding: 0; margin: 8px 0; }
  .post-list li {
    padding: 8px 12px;
    border-bottom: 1px solid #eee;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .post-title { font-weight: 500; }
  .post-meta { color: #888; font-size: 12px; }
  .pager-wrap { border-top: 1px solid #eee; padding: 4px 0; }
</style>
