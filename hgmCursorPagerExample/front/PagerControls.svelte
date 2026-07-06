{#if pager}
  {#if pager.notice}
    <div class="pager-notice">{pager.notice}</div>
  {/if}
  <div class="pager-row">
    <button class="btn btn-sm pager-icon"
      disabled={pager.loading}
      onclick={() => pager.refresh(false)}
      title="刷新当前页">↻</button>
    <button class="btn btn-sm"
      disabled={pager.loading || pager.currentPage <= 1}
      onclick={pager.goFirst}
      title="回到第 1 页">«首页</button>
    <button class="btn btn-sm"
      disabled={pager.loading || !pager.hasPrev}
      onclick={pager.goPrev}>上一页</button>
    <span class="pager-page">第 {pager.currentPage} 页</span>
    <button class="btn btn-sm"
      disabled={pager.loading || !pager.hasNext}
      onclick={pager.goNext}>下一页</button>
  </div>
{/if}

<script>
  // PagerControls.svelte — 游标分页器统一翻页 UI.
  //
  // 协议见 ../../doc/pager_design.md 第四章 "UI 组件常驻元素":
  //   * ↻ 刷新, «首页, 上一页, 第 X 页, 下一页 — 这 5 个常驻 (即使只有 1 页, 因为
  //     刷新按钮要在). disabled 由 loading + 页面状态决定.
  //   * notice 区: notice 非空时才显示, 与列表分开渲染.
  //
  // 用法: <PagerControls {pager} /> 其中 pager 来自 createCursorPager(...).
  let { pager } = $props()
</script>

<style>
  .pager-row {
    display: flex;
    gap: 4px;
    align-items: center;
    padding: 4px 0;
    flex-wrap: wrap;
  }
  .pager-row .btn {
    font-size: 12px;
    padding: 2px 8px;
  }
  .pager-page {
    font-size: 12px;
    color: #666;
    padding: 0 6px;
    white-space: nowrap;
    /* 保证 3 位页码 ("第 999 页") 不抖动. */
    min-width: 56px;
    text-align: center;
  }
  .pager-icon {
    min-width: 22px;
    padding: 2px 4px !important;
  }
  .pager-notice {
    padding: 4px 8px;
    font-size: 11px;
    color: #b07000;
    background: #fff7e0;
    border: 1px solid #f0e0a0;
    border-radius: 3px;
    margin: 4px 0;
  }
  .btn { padding: 4px 12px; border: 1px solid #ccc; background: #fff; border-radius: 4px; cursor: pointer; font-size: 13px; }
  .btn:hover { background: #f0f0f0; }
  .btn:disabled { color: #aaa; cursor: not-allowed; background: #f8f8f8; }
  .btn-sm { padding: 2px 8px; font-size: 12px; }
</style>
