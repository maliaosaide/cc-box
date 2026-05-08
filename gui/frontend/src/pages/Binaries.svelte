<script>
  import { onMount } from 'svelte'
  import { GetBinaryPage } from '../../wailsjs/go/main/App.js'

  export let syncState = 'idle'
  let activeTab = 'claude'
  let binData = null
  let loading = true
  let error = ''

  const tabs = [
    { id: 'claude', label: 'Claude', active: true },
    { id: 'codex', label: 'Codex', coming: true },
    { id: 'gemini', label: 'Gemini', coming: true },
  ]

  const placeholders = {
    codex: { name: 'OpenAI Codex CLI', desc: 'OpenAI 编码助手命令行工具' },
    gemini: { name: 'Google Gemini CLI', desc: 'Google Gemini 命令行工具' },
  }

  onMount(async () => {
    await loadBinary()
  })

  async function loadBinary() {
    loading = true; error = ''
    try { binData = await GetBinaryPage() }
    catch (e) { error = e.message || String(e) }
    loading = false
  }

  function formatSize(b) {
    if (!b) return '-'
    if (b < 1024) return b + ' B'
    if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
    return (b / 1024 / 1024).toFixed(1) + ' MB'
  }
</script>

<div class="bin-page">
  <h1 class="section-title animate-fade-in">二进制管理</h1>

  <div class="tabs-bar animate-fade-in">
    {#each tabs as tab}
      <button class="tab-btn" class:active={activeTab === tab.id}
              disabled={tab.coming}
              on:click={() => { if (tab.active) activeTab = tab.id }}>
        {tab.label}
        {#if tab.coming}
          <span class="coming-tag">即将支持</span>
        {/if}
      </button>
    {/each}
  </div>

  {#if activeTab !== 'claude'}
    <div class="card animate-fade-in">
      <div class="text-center py-16">
        <div class="empty-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="12" cy="12" r="10"/>
            <path d="M12 8v4m0 4h.01"/>
          </svg>
        </div>
        <p class="text-txt-primary text-sm font-medium mt-2">{placeholders[activeTab]?.name || activeTab}</p>
        <p class="text-txt-muted text-xs mt-1">{placeholders[activeTab]?.desc || '即将支持'}</p>
        <div class="mt-4">
          <span class="coming-badge">规划中</span>
        </div>
      </div>
    </div>
  {:else if loading}
    <div class="flex items-center justify-center h-64">
      <div class="loading-dot animate-gentle-pulse"></div>
    </div>
  {:else if error}
    <div class="card"><div class="empty-compact">{error}</div></div>
  {:else if binData}
    <div class="card animate-fade-in stagger-1">
      <div class="section-label-row">
        <span class="section-label">Claude</span>
        <span class="font-mono text-xs text-txt-muted">{binData.platform}</span>
      </div>
      <div class="current-row">
        <div class="item-badge accent">C</div>
        <div class="item-main">
          <span class="item-name">claude</span>
          <span class="item-detail font-mono">{binData.currentVersion || '未安装'}</span>
        </div>
        <span class="version-tag" class:latest={binData.localExists} class:!latest={!binData.localExists}>
          {binData.localExists ? '已安装' : '未安装'}
        </span>
      </div>
      {#if binData.binaryPath}
        <div class="path-row font-mono">{binData.binaryPath}</div>
      {/if}
    </div>

    {#if binData.versions && binData.versions.length > 0}
      <div class="card animate-fade-in stagger-2">
        <div class="section-label-row">
          <span class="section-label">版本列表</span>
          <span class="text-xs text-txt-muted">{binData.versions.length} 个版本</span>
        </div>
        <div class="item-list">
          {#each binData.versions as ver}
            <div class="item-row">
              <div class="ver-dot" class:current={ver.isCurrent}></div>
              <div class="item-main">
                <span class="item-name font-mono">{ver.version}</span>
                <span class="item-detail">{formatSize(ver.size)} · {ver.uploadedBy} · {ver.uploadedAt}</span>
              </div>
              <span class="version-tag" class:latest={ver.isCurrent}>
                {ver.isCurrent ? '当前' : '云端'}
              </span>
            </div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="card animate-fade-in stagger-2">
        <div class="empty-compact">暂无云端版本记录</div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .bin-page { display: flex; flex-direction: column; gap: 12px; }

  .tabs-bar {
    display: flex; gap: 2px; padding: 4px;
    background: rgb(var(--surface-1)); border-radius: 8px; border: 1px solid rgb(var(--border));
  }
  .tab-btn {
    flex: 1; padding: 6px 8px; border-radius: 6px;
    font-size: 12px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: transparent;
    border: none; cursor: pointer; transition: all 0.2s;
  }
  .tab-btn:hover { color: rgb(var(--text-secondary)); }
  .tab-btn.active { background: rgba(196,112,78,0.1); color: rgb(var(--accent)); }
  .tab-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .coming-tag {
    font-size: 9px; padding: 1px 5px; border-radius: 3px;
    background: rgba(196,112,78,0.08); color: rgb(var(--accent));
    margin-left: 4px; font-family: 'DM Mono', monospace;
  }
  .coming-badge {
    display: inline-block; font-size: 11px; font-family: 'DM Mono', monospace;
    padding: 3px 10px; border-radius: 4px;
    background: rgba(196,112,78,0.06); color: rgb(var(--accent));
    border: 1px solid rgba(196,112,78,0.15);
  }

  .section-label-row {
    display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px;
  }
  .section-label {
    font-size: 11px; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.05em; color: rgb(var(--text-muted));
  }

  .current-row {
    display: flex; align-items: center; gap: 10px; padding: 6px 0;
  }
  .item-badge {
    width: 28px; height: 28px; border-radius: 7px;
    display: flex; align-items: center; justify-content: center;
    font-size: 11px; font-family: 'Bricolage Grotesque', sans-serif;
    font-weight: 700; flex-shrink: 0;
  }
  .item-badge.accent { background: rgba(196,112,78,0.08); color: rgb(var(--accent)); }
  .item-main { flex: 1; min-width: 0; display: flex; align-items: baseline; gap: 8px; }
  .item-name { font-size: 12px; color: rgb(var(--text-primary)); }
  .item-detail { font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.7; }
  .version-tag {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 7px; border-radius: 3px;
    color: rgb(var(--text-muted)); background: rgb(var(--surface-2));
  }
  .version-tag.latest { background: rgba(107,144,128,0.1); color: rgb(var(--state-ok)); }

  .path-row {
    font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.5;
    padding: 4px 0 0 38px;
  }

  .item-list { display: flex; flex-direction: column; }
  .item-row {
    display: flex; align-items: center; gap: 10px; padding: 7px 0;
    border-bottom: 1px solid rgba(46,45,51,0.4);
  }
  .item-row:last-child { border-bottom: none; }

  .ver-dot {
    width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
    background: rgb(var(--text-muted)); opacity: 0.3;
  }
  .ver-dot.current { background: rgb(var(--state-ok)); opacity: 1; }

  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .empty-icon { width: 48px; height: 48px; margin: 0 auto 12px; color: rgb(var(--text-muted)); opacity: 0.4; }
  .empty-icon svg { width: 100%; height: 100%; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
</style>
