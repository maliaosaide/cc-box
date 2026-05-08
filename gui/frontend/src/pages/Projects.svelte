<script>
  import { onMount } from 'svelte'
  import { GetProjectList, BrowseFolder } from '../../wailsjs/go/main/App.js'

  export let syncState = 'idle'
  let data = null
  let loading = true
  let error = ''
  let expandedIdx = -1

  onMount(async () => {
    await loadProjects()
  })

  async function loadProjects() {
    loading = true; error = ''
    try { data = await GetProjectList() }
    catch (e) { error = e.message || String(e) }
    loading = false
  }

  function toggleExpand(i) {
    expandedIdx = expandedIdx === i ? -1 : i
  }
</script>

<div class="proj-page">
  <h1 class="section-title animate-fade-in">项目配置</h1>

  {#if loading}
    <div class="flex items-center justify-center h-64">
      <div class="loading-dot animate-gentle-pulse"></div>
    </div>
  {:else if error}
    <div class="card"><div class="empty-compact">{error}</div></div>
  {:else if !data || (data.projects.length === 0 && data.orphans.length === 0)}
    <div class="card animate-fade-in">
      <div class="text-center py-16">
        <div class="empty-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>
          </svg>
        </div>
        <p class="text-txt-primary text-sm font-medium mt-2">暂无已追踪的项目</p>
        <p class="text-txt-muted text-xs mt-1">确保项目目录下存在 .claude.json 文件</p>
      </div>
    </div>
  {:else}
    {#if data.projects && data.projects.length > 0}
      <div class="card animate-fade-in">
        <div class="section-label-row">
          <span class="section-label">已追踪项目</span>
          <span class="text-xs text-txt-muted">{data.projects.length} 个</span>
        </div>
        <div class="project-list">
          {#each data.projects as proj, i}
            <div class="proj-row" on:click={() => toggleExpand(i)}>
              <div class="proj-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="16" height="16">
                  <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>
                </svg>
              </div>
              <div class="proj-main">
                <span class="proj-name">{proj.name}</span>
                <span class="proj-remote font-mono">{proj.remote}</span>
              </div>
              <span class="proj-badge" class:active={proj.hasLocal}>
                {proj.hasLocal ? '本地' : '云端'}
              </span>
              <span class="proj-arrow" class:open={expandedIdx === i}>▶</span>
            </div>

            {#if expandedIdx === i}
              <div class="proj-detail animate-fade-in">
                <div class="detail-grid">
                  <div class="detail-item">
                    <span class="detail-label">项目路径</span>
                    <span class="detail-value font-mono">{proj.path}</span>
                  </div>
                  <div class="detail-item">
                    <span class="detail-label">Remote</span>
                    <span class="detail-value font-mono">{proj.remoteName}</span>
                  </div>
                  {#if proj.mcpCount > 0}
                    <div class="detail-item">
                      <span class="detail-label">MCP Servers</span>
                      <span class="detail-value">{proj.mcpCount} 个</span>
                    </div>
                  {/if}
                </div>
              </div>
            {/if}
          {/each}
        </div>
      </div>
    {/if}

    {#if data.orphans && data.orphans.length > 0}
      <div class="card animate-fade-in stagger-2">
        <div class="section-label-row">
          <span class="section-label">未匹配项目</span>
          <span class="text-xs text-txt-muted">{data.orphans.length} 个</span>
        </div>
        <div class="orphan-list">
          {#each data.orphans as orphan}
            <div class="orphan-row">
              <div class="orphan-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                  <circle cx="12" cy="12" r="10"/>
                  <path d="M12 8v4m0 4h.01"/>
                </svg>
              </div>
              <div class="orphan-main">
                <span class="orphan-remote font-mono">{orphan.remote}</span>
                <span class="orphan-desc">云端有配置但本地未找到对应项目</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .proj-page { display: flex; flex-direction: column; gap: 12px; }

  .section-label-row {
    display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px;
  }
  .section-label {
    font-size: 11px; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.05em; color: rgb(var(--text-muted));
  }

  .project-list { display: flex; flex-direction: column; }
  .proj-row {
    display: flex; align-items: center; gap: 10px;
    padding: 10px 0; border-bottom: 1px solid rgba(46,45,51,0.4);
    cursor: pointer; transition: all 0.2s;
  }
  .proj-row:last-child { border-bottom: none; }
  .proj-row:hover .proj-name { color: rgb(var(--accent)); }

  .proj-icon {
    width: 28px; height: 28px; border-radius: 7px;
    display: flex; align-items: center; justify-content: center;
    background: rgba(196,112,78,0.08); color: rgb(var(--accent));
    flex-shrink: 0;
  }
  .proj-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .proj-name {
    font-size: 13px; font-weight: 500; color: rgb(var(--text-primary));
    transition: color 0.2s;
  }
  .proj-remote {
    font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.7;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .proj-badge {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 7px; border-radius: 3px;
    color: rgb(var(--text-muted)); background: rgb(var(--surface-2));
  }
  .proj-badge.active {
    background: rgba(107,144,128,0.1); color: rgb(var(--state-ok));
  }
  .proj-arrow {
    font-size: 8px; color: rgb(var(--text-muted));
    transition: transform 0.2s;
  }
  .proj-arrow.open { transform: rotate(90deg); }

  .proj-detail {
    padding: 12px 0 12px 38px;
    border-bottom: 1px solid rgba(46,45,51,0.4);
  }

  .detail-grid {
    display: grid; grid-template-columns: 1fr 1fr; gap: 8px 24px;
  }
  .detail-item { display: flex; flex-direction: column; gap: 2px; }
  .detail-label {
    font-size: 10px; color: rgb(var(--text-muted));
    text-transform: uppercase; letter-spacing: 0.05em;
  }
  .detail-value { font-size: 12px; color: rgb(var(--text-primary)); }

  .orphan-list { display: flex; flex-direction: column; }
  .orphan-row {
    display: flex; align-items: center; gap: 10px;
    padding: 8px 0; border-bottom: 1px solid rgba(46,45,51,0.3);
  }
  .orphan-row:last-child { border-bottom: none; }
  .orphan-icon {
    width: 24px; height: 24px; border-radius: 6px;
    display: flex; align-items: center; justify-content: center;
    background: rgba(196,165,78,0.08); color: rgb(var(--state-warn));
    flex-shrink: 0;
  }
  .orphan-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .orphan-remote {
    font-size: 12px; color: rgb(var(--text-secondary));
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .orphan-desc { font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.6; }

  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .empty-icon { width: 48px; height: 48px; margin: 0 auto 12px; color: rgb(var(--text-muted)); opacity: 0.4; }
  .empty-icon svg { width: 100%; height: 100%; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
</style>
