<script>
  import { onMount } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { GetProjectList, GetProjectDetail, AddProjectPath, BrowseFolder, DeleteOrphan } from '../../wailsjs/go/main/App.js'

  export let active = false
  export let refreshToken = 0

  let data = null
  let loading = true
  let error = ''
  let expandedIdx = -1
  let detailJSON = ''
  let detailLoading = false
  let folderLoading = false
  let detailRequestId = 0
  let lastRefreshToken = 0

  $: if (active && refreshToken !== lastRefreshToken) {
    lastRefreshToken = refreshToken
    loadProjects()
  }

  $: projects = data?.projects || []
  $: orphans = data?.orphans || []

  onMount(async () => {
    await loadProjects()
    EventsOn('projects:updated', (result) => {
      data = { ...(result || {}), projects: result?.projects || [], orphans: result?.orphans || [] }
    })
  })

  async function loadProjects() {
    loading = true; error = ''
    try {
      const result = await GetProjectList()
      data = { ...(result || {}), projects: result?.projects || [], orphans: result?.orphans || [] }
    }
    catch (e) { error = e.message || String(e) }
    loading = false
  }

  async function toggleExpand(i, path) {
    if (expandedIdx === i) {
      detailRequestId += 1
      expandedIdx = -1; detailJSON = ''; detailLoading = false; return
    }
    const requestId = ++detailRequestId
    expandedIdx = i; detailJSON = ''; detailLoading = true
    try {
      const result = await GetProjectDetail(path)
      if (requestId === detailRequestId && expandedIdx === i) detailJSON = result
    } catch (e) {
      if (requestId === detailRequestId && expandedIdx === i) detailJSON = '加载失败: ' + (e.message || String(e))
    }
    if (requestId === detailRequestId && expandedIdx === i) detailLoading = false
  }

  async function addFolder() {
    folderLoading = true
    try {
      const dir = await BrowseFolder('选择项目目录')
      if (dir) {
        await AddProjectPath(dir)
        await loadProjects()
      }
    } catch (e) {
      error = e.message || String(e)
    }
    folderLoading = false
  }

  async function deleteOrphan(remote) {
    try {
      await DeleteOrphan(remote)
      await loadProjects()
    } catch (e) {
      error = e.message || String(e)
    }
  }
</script>

<div class="proj-page">
  <div class="toolbar animate-fade-in">
    <h1 class="section-title">项目配置</h1>
    <button class="btn-sm" disabled={folderLoading} on:click={addFolder}>
      {folderLoading ? '...' : '+ 添加项目'}
    </button>
  </div>

  {#if error}
    <div class="error-bar animate-fade-in">
      <span>{error}</span>
      <button class="link-btn" on:click={() => error = ''}>关闭</button>
    </div>
  {/if}

  {#if loading}
    <div class="flex items-center justify-center h-64">
      <div class="loading-dot animate-gentle-pulse"></div>
    </div>
  {:else if !data || (projects.length === 0 && orphans.length === 0)}
    <div class="card animate-fade-in">
      <div class="text-center py-16">
        <div class="empty-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>
          </svg>
        </div>
        <p class="text-txt-primary text-sm font-medium mt-2">暂无已追踪的项目</p>
        <p class="text-txt-muted text-xs mt-1">点击右上角添加包含 .claude.json 的项目目录</p>
      </div>
    </div>
  {:else}
    {#if projects && projects.length > 0}
      <div class="card animate-fade-in">
        <div class="section-label-row">
          <span class="section-label">已追踪项目</span>
          <span class="text-xs text-txt-muted">{projects.length} 个</span>
        </div>
        <div class="project-list">
          {#each projects as proj, i}
            <button class="proj-row" type="button" on:click={() => toggleExpand(i, proj.path)}>
              <div class="proj-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="16" height="16">
                  <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>
                </svg>
              </div>
              <div class="proj-main">
                <span class="proj-name">{proj.name}</span>
                <span class="proj-remote font-mono">{proj.remote}</span>
              </div>
              {#if proj.mcpCount > 0}
                <span class="proj-mcp">{proj.mcpCount} MCP</span>
              {/if}
              <span class="proj-arrow" class:open={expandedIdx === i}>▶</span>
            </button>

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
                </div>
                {#if detailLoading}
                  <div class="empty-compact" style="margin-top:8px">加载 .claude.json...</div>
                {:else if detailJSON}
                  <div class="json-preview">
                    <div class="json-header">.claude.json</div>
                    <pre class="json-content">{detailJSON}</pre>
                  </div>
                {/if}
              </div>
            {/if}
          {/each}
        </div>
      </div>
    {/if}

    {#if orphans && orphans.length > 0}
      <div class="card animate-fade-in stagger-2">
        <div class="section-label-row">
          <span class="section-label">未匹配项目</span>
          <span class="text-xs text-txt-muted">{orphans.length} 个</span>
        </div>
        <div class="orphan-list">
          {#each orphans as orphan}
            <div class="orphan-row">
              <div class="orphan-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                  <circle cx="12" cy="12" r="10"/>
                  <path d="M12 8v4m0 4h.01"/>
                </svg>
              </div>
              <div class="orphan-main">
                <span class="orphan-remote font-mono">{orphan.remote}</span>
                <span class="orphan-desc">云端有配置但本地未找到</span>
              </div>
              <button class="btn-del" on:click|stopPropagation={() => deleteOrphan(orphan.remote)}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .proj-page { display: flex; flex-direction: column; gap: 12px; }
  .toolbar { display: flex; align-items: center; justify-content: space-between; }

  .section-label-row {
    display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px;
  }
  .section-label {
    font-size: 11px; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.05em; color: rgb(var(--text-muted));
  }

  .project-list { display: flex; flex-direction: column; }
  .proj-row {
    display: flex; align-items: center; gap: 10px; width: 100%;
    padding: 10px 0; border: 0; border-bottom: 1px solid rgba(46,45,51,0.4);
    background: transparent; color: inherit; font: inherit; text-align: left;
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
  .proj-mcp {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 7px; border-radius: 3px;
    background: rgba(107,144,128,0.1); color: rgb(var(--state-ok));
    flex-shrink: 0;
  }
  .proj-arrow {
    font-size: 8px; color: rgb(var(--text-muted));
    transition: transform 0.2s; flex-shrink: 0;
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

  .json-preview {
    margin-top: 10px; border-radius: 6px;
    background: rgb(var(--surface-2)); overflow: hidden;
  }
  .json-header {
    font-size: 10px; font-weight: 600; padding: 6px 10px;
    color: rgb(var(--text-muted)); text-transform: uppercase; letter-spacing: 0.05em;
    border-bottom: 1px solid rgb(var(--border));
  }
  .json-content {
    font-size: 11px; font-family: 'DM Mono', monospace;
    color: rgb(var(--text-secondary)); padding: 10px;
    margin: 0; max-height: 200px; overflow-y: auto;
    white-space: pre-wrap; word-break: break-all;
  }

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

  .btn-sm {
    font-size: 11px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--accent));
    background: rgba(196,112,78,0.08);
    border: 1px solid rgba(196,112,78,0.2);
    padding: 5px 12px; border-radius: 5px; cursor: pointer;
    transition: all 0.2s; white-space: nowrap;
  }
  .btn-sm:hover { background: rgba(196,112,78,0.15); }
  .btn-sm:disabled { opacity: 0.4; cursor: not-allowed; }

  .btn-del {
    width: 24px; height: 24px; border-radius: 4px;
    display: flex; align-items: center; justify-content: center;
    background: transparent; border: none; cursor: pointer;
    color: rgb(var(--text-muted)); opacity: 0.3; transition: all 0.2s;
    flex-shrink: 0;
  }
  .btn-del:hover { opacity: 1; color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }

  .error-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(184,92,92,0.08); border: 1px solid rgba(184,92,92,0.15);
    font-size: 12px; color: rgb(var(--state-err));
  }
  .link-btn {
    font-size: 11px; color: rgb(var(--text-muted));
    background: none; border: none; cursor: pointer;
  }
  .link-btn:hover { color: rgb(var(--accent)); }

  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .empty-icon { width: 48px; height: 48px; margin: 0 auto 12px; color: rgb(var(--text-muted)); opacity: 0.4; }
  .empty-icon svg { width: 100%; height: 100%; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
</style>
