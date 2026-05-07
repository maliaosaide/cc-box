<script>
  export let syncState = 'idle'
  let activeTab = 'claude'

  const binaries = [
    { id: 'claude', label: 'Claude', active: true },
    { id: 'codex', label: 'Codex', active: false, coming: true },
    { id: 'gemini', label: 'Gemini', active: false, coming: true },
  ]

  const placeholder = {
    codex: { name: 'OpenAI Codex CLI', desc: 'OpenAI 编码助手命令行工具' },
    gemini: { name: 'Google Gemini CLI', desc: 'Google Gemini 命令行工具' },
  }
</script>

<div class="space-y-6">
  <h1 class="section-title">二进制管理</h1>

  <div class="flex gap-1 bg-surface-1 rounded-lg p-1 border border-bdr">
    {#each binaries as bin}
      <button class="tab-btn" class:active={activeTab === bin.id}
              disabled={bin.coming}
              on:click={() => { if (bin.active) activeTab = bin.id }}>
        {bin.label}
        {#if bin.coming}
          <span class="coming-tag">即将支持</span>
        {/if}
      </button>
    {/each}
  </div>

  {#if activeTab === 'claude'}
    <div class="card flex items-center justify-center py-20">
      <div class="text-center">
        <div class="empty-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
          </svg>
        </div>
        <p class="text-txt-muted text-sm">Claude 版本管理</p>
        <p class="text-txt-muted text-xs mt-1 font-mono opacity-50">Phase 3c</p>
      </div>
    </div>
  {:else}
    <div class="card">
      <div class="text-center py-16">
        <div class="empty-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="12" cy="12" r="10"/>
            <path d="M12 8v4m0 4h.01"/>
          </svg>
        </div>
        <p class="text-txt-primary text-sm font-medium mt-2">{placeholder[activeTab]?.name || activeTab}</p>
        <p class="text-txt-muted text-xs mt-1">{placeholder[activeTab]?.desc || '即将支持'}</p>
        <div class="mt-4">
          <span class="coming-badge">规划中</span>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .coming-tag {
    font-size: 9px;
    padding: 1px 5px;
    border-radius: 3px;
    background: rgba(196,112,78,0.08);
    color: rgb(var(--accent));
    margin-left: 4px;
    vertical-align: middle;
    font-family: 'DM Mono', monospace;
  }

  .coming-badge {
    display: inline-block;
    font-size: 11px;
    font-family: 'DM Mono', monospace;
    padding: 3px 10px;
    border-radius: 4px;
    background: rgba(196,112,78,0.06);
    color: rgb(var(--accent));
    border: 1px solid rgba(196,112,78,0.15);
  }

  .empty-icon {
    width: 48px; height: 48px; margin: 0 auto 12px;
    color: rgb(var(--text-muted)); opacity: 0.4;
  }
  .empty-icon svg { width: 100%; height: 100%; }
</style>
