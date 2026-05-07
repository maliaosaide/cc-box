<script>
  export let syncState = 'idle'
</script>

<div class="space-y-6 max-w-4xl">
  <h1 class="section-title">历史记录</h1>

  <div class="card flex items-center justify-center py-20">
    <div class="text-center">
      <div class="empty-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="12" cy="12" r="10"/>
          <path d="M12 6v6l4 2"/>
        </svg>
      </div>
      <p class="text-txt-muted text-sm">快照时间线</p>
      <p class="text-txt-muted text-xs mt-1 font-mono opacity-50">Phase 3d</p>
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
