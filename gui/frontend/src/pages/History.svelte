<script>
  import { onMount } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { GetLocalSnapshotList, GetSnapshotList, GetSnapshotDetail, RevertToSnapshot } from '../../wailsjs/go/main/App.js'

  export let syncState = 'idle'

  let snapshots = []
  let filtered = []
  let loading = true
  let error = ''
  let msg = ''
  let expandedId = ''
  let detail = null
  let detailLoading = false
  let loadCount = 20
  let deviceFilter = ''
  let devices = []

  onMount(async () => {
    await refresh()
    EventsOn('op:complete', () => refresh())
  })

  async function refresh() {
    loading = true; error = ''
    try {
      snapshots = await GetLocalSnapshotList(loadCount) || []
      if (!snapshots.length) {
        snapshots = await GetSnapshotList(loadCount) || []
      }
      const devSet = new Set()
      snapshots.forEach(s => { if (s.device) devSet.add(s.device) })
      devices = [...devSet]
      applyFilter()
    } catch (e) {
      error = e.message || String(e)
    }
    loading = false
  }

  function applyFilter() {
    if (!deviceFilter) { filtered = snapshots; return }
    filtered = snapshots.filter(s => s.device === deviceFilter)
  }

  async function loadMore() {
    loadCount += 20
    await refresh()
  }

  async function toggleDetail(id) {
    if (expandedId === id) {
      expandedId = ''; detail = null; return
    }
    expandedId = id; detail = null; detailLoading = true
    try {
      detail = await GetSnapshotDetail(id)
    } catch (e) {
      detail = null
    }
    detailLoading = false
  }

  async function rollback(id) {
    msg = ''; error = ''
    try {
      await RevertToSnapshot(id)
      msg = '已回滚到快照 ' + id.slice(0, 12)
      await refresh()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  function formatSize(b) {
    if (!b) return '-'
    if (b < 1024) return b + ' B'
    if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
    return (b / 1024 / 1024).toFixed(1) + ' MB'
  }
</script>

<div class="history-page">
  <div class="toolbar animate-fade-in">
    <h1 class="section-title">历史记录</h1>
    {#if snapshots.length > 0}
      <span class="text-xs text-txt-muted font-mono">{snapshots.length} 条记录</span>
    {/if}
  </div>

  {#if msg}
    <div class="msg-bar animate-fade-in">
      <span>{msg}</span>
      <button class="link-btn" on:click={() => msg = ''}>关闭</button>
    </div>
  {/if}

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
  {:else if snapshots.length === 0}
    <div class="card flex items-center justify-center py-20">
      <div class="text-center">
        <div class="empty-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="12" cy="12" r="10"/>
            <path d="M12 6v6l4 2"/>
          </svg>
        </div>
        <p class="text-txt-muted text-sm">暂无快照记录</p>
        <p class="text-txt-muted text-xs mt-1">推送配置后将自动创建快照</p>
      </div>
    </div>
  {:else}
    <!-- 筛选栏 -->
    {#if devices.length > 1}
      <div class="filter-bar animate-fade-in">
        <button class="filter-btn" class:active={!deviceFilter} on:click={() => { deviceFilter = ''; applyFilter() }}>
          全部
        </button>
        {#each devices as dev}
          <button class="filter-btn" class:active={deviceFilter === dev} on:click={() => { deviceFilter = dev; applyFilter() }}>
            {dev}
          </button>
        {/each}
      </div>
    {/if}

    <div class="timeline">
      {#each filtered as snap, i}
        <div class="timeline-item animate-fade-in" style="animation-delay: {i * 40}ms">
          <div class="timeline-node" class:active={expandedId === snap.id}>
            <div class="timeline-dot"></div>
            {#if i < filtered.length - 1}
              <div class="timeline-line"></div>
            {/if}
          </div>

          <div class="timeline-content">
            <div class="snap-row" on:click={() => toggleDetail(snap.id)}>
              <div class="snap-main">
                <span class="snap-id font-mono">{snap.shortId}</span>
                <span class="snap-time">{snap.timestamp}</span>
                <span class="snap-device">{snap.device}</span>
              </div>
              <div class="snap-meta">
                <span class="snap-message">{snap.message}</span>
                <span class="snap-files">{snap.fileCount} 文件</span>
              </div>
              <span class="snap-arrow" class:open={expandedId === snap.id}>▶</span>
            </div>

            {#if expandedId === snap.id}
              <div class="snap-detail animate-fade-in">
                {#if detailLoading}
                  <div class="empty-compact">加载中...</div>
                {:else if detail}
                  <div class="detail-grid">
                    <div class="detail-item">
                      <span class="detail-label">快照 ID</span>
                      <span class="detail-value font-mono">{detail.id}</span>
                    </div>
                    <div class="detail-item">
                      <span class="detail-label">设备</span>
                      <span class="detail-value">{detail.device}</span>
                    </div>
                    <div class="detail-item">
                      <span class="detail-label">消息</span>
                      <span class="detail-value">{detail.message}</span>
                    </div>
                    {#if detail.parent}
                      <div class="detail-item">
                        <span class="detail-label">父快照</span>
                        <span class="detail-value font-mono">{detail.parent.slice(0, 12)}</span>
                      </div>
                    {/if}
                  </div>

                  {#if detail.binary && Object.keys(detail.binary).length > 0}
                    <div class="detail-section">
                      <span class="detail-section-label">二进制版本</span>
                      <div class="binary-tags">
                        {#each Object.entries(detail.binary) as [platform, bins]}
                          {#each Object.entries(bins) as [name, version]}
                            <span class="binary-tag">{name} {version}</span>
                          {/each}
                        {/each}
                      </div>
                    </div>
                  {/if}

                  {#if detail.files && Object.keys(detail.files).length > 0}
                    <div class="detail-section">
                      <span class="detail-section-label">文件列表 ({Object.keys(detail.files).length})</span>
                      <div class="file-list">
                        {#each Object.entries(detail.files).slice(0, 20) as [path, entry]}
                          <div class="file-row">
                            <span class="file-path font-mono">{path}</span>
                            <span class="file-size">{formatSize(entry.size)}</span>
                          </div>
                        {/each}
                        {#if Object.keys(detail.files).length > 20}
                          <div class="empty-compact">... 还有 {Object.keys(detail.files).length - 20} 个文件</div>
                        {/if}
                      </div>
                    </div>
                  {/if}

                  <div class="detail-actions">
                    <button class="btn-sm" on:click|stopPropagation={() => rollback(snap.id)}>
                      回滚到此版本
                    </button>
                  </div>
                {:else}
                  <div class="empty-compact">无法加载详情</div>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      {/each}

      <div class="load-more">
        <button class="btn-ghost" on:click={loadMore}>加载更多</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .history-page { display: flex; flex-direction: column; gap: 12px; }
  .toolbar { display: flex; align-items: center; justify-content: space-between; }

  .msg-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(107,144,128,0.08); border: 1px solid rgba(107,144,128,0.15);
    font-size: 12px; color: rgb(var(--state-ok));
  }
  .error-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(184,92,92,0.08); border: 1px solid rgba(184,92,92,0.15);
    font-size: 12px; color: rgb(var(--state-err));
  }
  .link-btn { font-size: 11px; color: rgb(var(--text-muted)); background: none; border: none; cursor: pointer; }
  .link-btn:hover { color: rgb(var(--accent)); }

  .filter-bar {
    display: flex; gap: 2px; padding: 4px;
    background: rgb(var(--surface-1)); border-radius: 8px; border: 1px solid rgb(var(--border));
  }
  .filter-btn {
    padding: 4px 10px; border-radius: 5px;
    font-size: 11px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: transparent;
    border: 1px solid transparent; cursor: pointer; transition: all 0.2s;
  }
  .filter-btn:hover { color: rgb(var(--text-secondary)); background: rgb(var(--surface-2)); }
  .filter-btn.active { background: rgba(196,112,78,0.1); color: rgb(var(--accent)); border-color: rgba(196,112,78,0.2); }

  .timeline { display: flex; flex-direction: column; }
  .timeline-item { display: flex; gap: 16px; position: relative; }

  .timeline-node {
    display: flex; flex-direction: column; align-items: center;
    width: 20px; flex-shrink: 0; padding-top: 12px;
  }
  .timeline-dot {
    width: 8px; height: 8px; border-radius: 50%;
    background: rgb(var(--accent)); opacity: 0.5;
    flex-shrink: 0; z-index: 1;
  }
  .timeline-node.active .timeline-dot { opacity: 1; background: rgb(var(--accent-bright)); }
  .timeline-line {
    width: 1px; flex: 1; min-height: 20px;
    background: rgb(var(--border)); margin-top: 4px;
  }

  .timeline-content { flex: 1; min-width: 0; padding-bottom: 4px; }

  .snap-row {
    display: flex; align-items: center; gap: 12px;
    padding: 10px 14px; border-radius: 8px;
    background: rgb(var(--surface-1)); border: 1px solid rgb(var(--border));
    cursor: pointer; transition: all 0.2s;
  }
  .snap-row:hover { border-color: rgba(196,112,78,0.3); }

  .snap-main { display: flex; align-items: center; gap: 10px; flex: 1; min-width: 0; }
  .snap-id { font-size: 12px; color: rgb(var(--accent)); }
  .snap-time { font-size: 12px; color: rgb(var(--text-secondary)); }
  .snap-device { font-size: 11px; color: rgb(var(--text-muted)); }

  .snap-meta { display: flex; align-items: center; gap: 8px; }
  .snap-message { font-size: 11px; color: rgb(var(--text-muted)); max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .snap-files { font-size: 10px; font-family: 'DM Mono', monospace; color: rgb(var(--text-muted)); opacity: 0.6; }

  .snap-arrow { font-size: 8px; color: rgb(var(--text-muted)); transition: transform 0.2s; }
  .snap-arrow.open { transform: rotate(90deg); }

  .snap-detail {
    margin-top: 6px; padding: 14px;
    background: rgb(var(--surface-1)); border: 1px solid rgb(var(--border));
    border-radius: 8px; border-top-left-radius: 0;
  }

  .detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 24px; margin-bottom: 12px; }
  .detail-item { display: flex; flex-direction: column; gap: 2px; }
  .detail-label { font-size: 10px; color: rgb(var(--text-muted)); text-transform: uppercase; letter-spacing: 0.05em; }
  .detail-value { font-size: 12px; color: rgb(var(--text-primary)); }

  .detail-section { margin-top: 12px; }
  .detail-section-label { font-size: 11px; font-weight: 600; color: rgb(var(--text-muted)); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 8px; display: block; }

  .detail-actions {
    margin-top: 12px; padding-top: 10px;
    border-top: 1px solid rgb(var(--border));
    display: flex; justify-content: flex-end;
  }
  .btn-sm {
    font-size: 11px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--accent));
    background: rgba(196,112,78,0.08);
    border: 1px solid rgba(196,112,78,0.2);
    padding: 5px 12px; border-radius: 5px; cursor: pointer;
    transition: all 0.2s;
  }
  .btn-sm:hover { background: rgba(196,112,78,0.15); }

  .binary-tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .binary-tag {
    font-size: 11px; font-family: 'DM Mono', monospace;
    padding: 3px 8px; border-radius: 4px;
    background: rgba(196,112,78,0.08); color: rgb(var(--accent));
  }

  .file-list { display: flex; flex-direction: column; gap: 2px; max-height: 200px; overflow-y: auto; }
  .file-row { display: flex; align-items: center; justify-content: space-between; padding: 3px 0; }
  .file-path { font-size: 11px; color: rgb(var(--text-secondary)); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .file-size { font-size: 10px; font-family: 'DM Mono', monospace; color: rgb(var(--text-muted)); flex-shrink: 0; }

  .load-more { display: flex; justify-content: center; padding: 16px 0 0 36px; }
  .btn-ghost {
    font-size: 12px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-secondary)); background: rgb(var(--surface-2));
    border: 1px solid rgb(var(--border));
    padding: 6px 14px; border-radius: 6px; cursor: pointer; transition: all 0.2s;
  }
  .btn-ghost:hover { border-color: rgb(var(--accent)); color: rgb(var(--accent)); }

  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .empty-icon { width: 48px; height: 48px; margin: 0 auto 12px; color: rgb(var(--text-muted)); opacity: 0.4; }
  .empty-icon svg { width: 100%; height: 100%; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
</style>
