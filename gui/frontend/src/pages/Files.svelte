<script>
  export let syncState = 'idle'
  import { createEventDispatcher } from 'svelte'
  const dispatch = createEventDispatcher()
</script>

<div class="space-y-6 max-w-5xl">
  <div class="flex items-center justify-between">
    <h1 class="section-title">配置文件</h1>
    <div class="flex gap-2">
      <button class="btn-ghost text-xs">推送所有</button>
      <button class="btn-ghost text-xs">拉取所有</button>
    </div>
  </div>

  <div class="flex gap-2">
    <button class="btn-preset active">全部</button>
    <button class="btn-preset">已变更</button>
    <button class="btn-preset">冲突</button>
  </div>

  <div class="card flex items-center justify-center py-20">
    <div class="text-center">
      <div class="empty-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M3 3h5v5H3V3zm9 0h5v5h-5V3zm-9 9h5v5H3v-5zm9 0h5v5h-5v-5z"/>
        </svg>
      </div>
      <p class="text-txt-muted text-sm">文件管理页面</p>
      <p class="text-txt-muted text-xs mt-1 font-mono opacity-50">Phase 3b</p>
    </div>
  </div>
</div>

<style>
  .empty-icon {
    width: 48px; height: 48px; margin: 0 auto 12px;
    color: rgb(var(--text-muted)); opacity: 0.4;
  }
  .empty-icon svg { width: 100%; height: 100%; }
</style>
