<script>
  export let syncState = 'idle'
  let activeTab = 'connection'

  const tabs = [
    { id: 'connection', label: '连接' },
    { id: 'encryption', label: '加密' },
    { id: 'sync', label: '同步' },
    { id: 'paths', label: '路径' },
    { id: 'exclude', label: '排除规则' },
    { id: 'devices', label: '设备' },
  ]
</script>

<div class="space-y-6 max-w-3xl">
  <h1 class="section-title">设置</h1>

  <div class="flex gap-1 bg-surface-1 rounded-lg p-1 border border-bdr">
    {#each tabs as tab}
      <button class="tab-btn" class:active={activeTab === tab.id}
              on:click={() => activeTab = tab.id}>
        {tab.label}
      </button>
    {/each}
  </div>

  <div class="card flex items-center justify-center py-20">
    <div class="text-center">
      <div class="empty-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M12.22 2h-.44a2 2 0 00-2 2v.18a2 2 0 01-1 1.73l-.43.25a2 2 0 01-2 0l-.15-.08a2 2 0 00-2.73.73l-.22.38a2 2 0 00.73 2.73l.15.1a2 2 0 011 1.72v.51a2 2 0 01-1 1.74l-.15.09a2 2 0 00-.73 2.73l.22.38a2 2 0 002.73.73l.15-.08a2 2 0 012 0l.43.25a2 2 0 011 1.73V20a2 2 0 002 2h.44a2 2 0 002-2v-.18a2 2 0 011-1.73l.43-.25a2 2 0 012 0l.15.08a2 2 0 002.73-.73l.22-.39a2 2 0 00-.73-2.73l-.15-.08a2 2 0 01-1-1.74v-.5a2 2 0 011-1.74l.15-.09a2 2 0 00.73-2.73l-.22-.38a2 2 0 00-2.73-.73l-.15.08a2 2 0 01-2 0l-.43-.25a2 2 0 01-1-1.73V4a2 2 0 00-2-2z"/>
          <circle cx="12" cy="12" r="3"/>
        </svg>
      </div>
      <p class="text-txt-muted text-sm">设置页面</p>
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
