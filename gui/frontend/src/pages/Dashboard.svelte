<script>
  import { onMount, createEventDispatcher } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { GetDashboard, QuickPush, QuickPull, QuickSync } from '../../wailsjs/go/main/App.js'

  export let syncState = 'idle'
  const dispatch = createEventDispatcher()

  let dashboard = null
  let loading = true
  let actionLoading = ''
  let progress = null

  onMount(async () => {
    await refresh()
    EventsOn('op:progress', (e) => {
      if (e.operation && e.operation.startsWith('quick-')) progress = e
    })
    EventsOn('op:complete', () => {
      actionLoading = ''; progress = null; syncState = 'synced'; refresh()
    })
  })

  async function refresh() {
    try {
      dashboard = await GetDashboard()
      if (dashboard && dashboard.conflicts) syncState = 'conflict'
      else if (dashboard && dashboard.lastSync) syncState = 'synced'
      else syncState = 'idle'
    } catch (e) { syncState = 'error' }
    loading = false
  }

  async function doAction(action) {
    actionLoading = action; progress = null; syncState = 'syncing'
    try {
      if (action === 'push') QuickPush()
      else if (action === 'pull') QuickPull()
      else QuickSync()
    } catch (e) { console.error(action, e) }
  }

  function navigateTo(page) { dispatch('navigate', { page }) }

  $: data = dashboard || {
    syncStatus: 'idle', lastSync: null, claudeVersion: '-',
    claudeLatest: true, conflicts: 0, devices: [], recentChanges: [],
    backups: [], binaries: []
  }
  $: hasConflicts = data.conflicts !== 0
  $: hasChanges = data.recentChanges && data.recentChanges.length
  $: hasBackups = data.backups && data.backups.length
  $: hasBinaries = data.binaries && data.binaries.length
  $: hasDevices = data.devices && data.devices.length > 1
</script>

{#if loading}
  <div class="flex items-center justify-center h-64">
    <div class="loading-dot animate-gentle-pulse"></div>
  </div>
{:else}
  <div class="dash">
    <!-- 顶部操作条 -->
    <div class="toolbar animate-fade-in">
      <h1 class="section-title">概览</h1>
      <div class="toolbar-right">
        <div class="status-pill" class:conflict={hasConflicts}>
          <div class="status-dot" class:ok={!hasConflicts} class:err={hasConflicts}></div>
          <span class="status-text">
            {!hasConflicts ? '已同步' : `${data.conflicts} 个冲突`}
          </span>
          {#if hasConflicts}
            <button class="status-link" on:click={() => navigateTo('files')}>解决</button>
          {/if}
        </div>
        <div class="toolbar-divider"></div>
        <div class="action-group">
          <button class="action-btn" disabled={actionLoading === 'push'} on:click={() => doAction('push')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 19V5m-7 7l7-7 7 7"/></svg>
            <span>推送</span>
          </button>
          <button class="action-btn" disabled={actionLoading === 'pull'} on:click={() => doAction('pull')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14m-7-7l7 7 7-7"/></svg>
            <span>拉取</span>
          </button>
          <button class="action-btn" disabled={actionLoading === 'sync'} on:click={() => doAction('sync')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M1 4v6h6M23 20v-6h-6"/>
              <path d="M20.49 9A9 9 0 005.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 013.51 15"/>
            </svg>
            <span>同步</span>
          </button>
        </div>
      </div>
    </div>

    {#if progress}
      <div class="progress-section animate-fade-in">
        <div class="flex justify-between text-xs mb-1">
          <span class="font-mono text-txt-muted">{progress.message}</span>
          <span class="font-mono text-txt-secondary">{Math.round(progress.percent)}%</span>
        </div>
        <div class="progress-bar">
          <div class="progress-bar-fill" style="width: {progress.percent}%"></div>
        </div>
      </div>
    {/if}

    <!-- 版本信息（全宽） -->
    <div class="card animate-fade-in stagger-1">
      <div class="section-label-row">
        <span class="section-label">版本</span>
        <button class="link-btn" on:click={() => navigateTo('binaries')}>管理二进制</button>
      </div>
      <div class="item-list">
        {#if hasBinaries}
          {#each data.binaries as bin}
            <div class="item-row">
              <div class="item-badge accent">{bin.name[0].toUpperCase()}</div>
              <div class="item-main">
                <span class="item-name">{bin.name}</span>
                <span class="item-detail font-mono">{bin.version || '未安装'}</span>
              </div>
              <span class="version-tag" class:latest={bin.latest}>
                {bin.installed ? (bin.latest ? '最新' : '可更新') : '未安装'}
              </span>
            </div>
          {/each}
        {:else}
          <div class="item-row">
            <div class="item-badge accent">C</div>
            <div class="item-main">
              <span class="item-name">claude</span>
              <span class="item-detail font-mono">{data.claudeVersion || '未安装'}</span>
            </div>
            <span class="version-tag" class:latest={data.claudeLatest}>
              {data.claudeLatest ? '最新' : '可更新'}
            </span>
          </div>
        {/if}
      </div>
    </div>

    <!-- 备份快照（全宽，含设备信息） -->
    <div class="card animate-fade-in stagger-2">
      <div class="section-label-row">
        <span class="section-label">备份</span>
        <button class="link-btn" on:click={() => navigateTo('history')}>全部历史</button>
      </div>
      {#if hasBackups}
        <div class="item-list">
          {#each data.backups as backup}
            <div class="item-row">
              <div class="item-badge dot-badge">
                <div class="backup-dot"></div>
              </div>
              <div class="item-main">
                <span class="item-name">{backup.message}</span>
                <span class="item-detail">
                  {backup.device} · {backup.time}
                </span>
              </div>
              <button class="link-btn" on:click={() => navigateTo('history')}>查看</button>
            </div>
          {/each}
        </div>
      {:else}
        <div class="empty-compact">暂无备份快照</div>
      {/if}
    </div>

    <!-- 最近变更（全宽） -->
    <div class="card animate-fade-in stagger-3">
      <div class="section-label-row">
        <span class="section-label">最近变更</span>
        <button class="link-btn" on:click={() => navigateTo('history')}>全部</button>
      </div>
      {#if hasChanges}
        <div class="item-list">
          {#each data.recentChanges as change}
            <div class="item-row">
              <div class="change-letter"
                   class:added={change.status === 'A'}
                   class:modified={change.status === 'M'}
                   class:conflict-c={change.status === 'C'}>
                {change.status}
              </div>
              <div class="item-main">
                <span class="item-name font-mono">{change.path}</span>
                <span class="item-detail">{change.time}</span>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <div class="empty-compact">暂无变更记录</div>
      {/if}
    </div>

    <!-- 设备列表 -->
    {#if hasDevices}
      <div class="card animate-fade-in stagger-4">
        <div class="section-label-row">
          <span class="section-label">设备</span>
        </div>
        <div class="item-list">
          {#each data.devices as dev}
            <div class="item-row">
              <div class="item-badge accent">{dev.name ? dev.name[0].toUpperCase() : '?'}</div>
              <div class="item-main">
                <span class="item-name">{dev.name || dev.platform}</span>
                <span class="item-detail">
                  {dev.platform}{dev.isCurrent ? ' · 本机' : ''} · {dev.lastActive || '未知'}
                </span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .dash {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  /* 顶部操作条 */
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .status-pill {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 12px;
    border-radius: 8px;
    background: rgb(var(--surface-2));
  }
  .status-pill.conflict { background: rgba(184,92,92,0.06); }
  .status-dot { width: 6px; height: 6px; border-radius: 50%; }
  .status-dot.ok { background: rgb(var(--state-ok)); }
  .status-dot.err { background: rgb(var(--state-err)); }
  .status-text { font-size: 12px; font-family: 'DM Mono', monospace; color: rgb(var(--text-secondary)); }
  .status-link {
    font-size: 11px; color: rgb(var(--state-err)); background: none;
    border: none; cursor: pointer; margin-left: 4px; transition: color 0.2s;
  }
  .status-link:hover { color: rgb(var(--accent)); }
  .toolbar-divider { width: 1px; height: 20px; background: rgb(var(--border)); }
  .action-group { display: flex; gap: 4px; }
  .action-btn {
    display: flex; align-items: center; gap: 4px;
    padding: 5px 10px; border-radius: 6px;
    font-size: 12px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-secondary));
    background: rgb(var(--surface-2));
    border: 1px solid rgb(var(--border));
    cursor: pointer; transition: all 0.25s ease-out;
  }
  .action-btn svg { width: 14px; height: 14px; }
  .action-btn:hover {
    border-color: rgba(196,112,78,0.4); color: rgb(var(--accent));
    background: rgba(196,112,78,0.05);
  }
  .action-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  /* 通用 */
  .section-label {
    font-size: 11px; font-weight: 600;
    text-transform: uppercase; letter-spacing: 0.05em;
    color: rgb(var(--text-muted));
  }
  .section-label-row {
    display: flex; align-items: center;
    justify-content: space-between; margin-bottom: 8px;
  }
  .link-btn {
    font-size: 11px; color: rgb(var(--text-muted));
    background: none; border: none; cursor: pointer; transition: color 0.2s;
  }
  .link-btn:hover { color: rgb(var(--accent)); }
  .empty-compact {
    text-align: center; padding: 14px 0;
    color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6;
  }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
  .progress-section { margin-top: -4px; }

  /* 统一列表行 */
  .item-list { display: flex; flex-direction: column; }
  .item-row {
    display: flex; align-items: center; gap: 10px;
    padding: 7px 0;
    border-bottom: 1px solid rgba(46,45,51,0.4);
  }
  .item-row:last-child { border-bottom: none; }

  .item-badge {
    width: 28px; height: 28px; border-radius: 7px;
    display: flex; align-items: center; justify-content: center;
    font-size: 11px; font-family: 'Bricolage Grotesque', sans-serif;
    font-weight: 700; flex-shrink: 0;
  }
  .item-badge.accent {
    background: rgba(196,112,78,0.08); color: rgb(var(--accent));
  }
  .item-badge.dot-badge {
    background: rgba(196,112,78,0.06);
  }

  .backup-dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: rgb(var(--accent)); opacity: 0.5;
  }

  .item-main {
    flex: 1; min-width: 0;
    display: flex; align-items: baseline; gap: 8px;
  }

  .item-name {
    font-size: 12px; color: rgb(var(--text-primary));
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .item-detail {
    font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.7;
    white-space: nowrap;
  }

  .version-tag {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 7px; border-radius: 3px;
    color: rgb(var(--text-muted)); background: rgb(var(--surface-2));
    white-space: nowrap;
  }
  .version-tag.latest {
    background: rgba(107,144,128,0.1); color: rgb(var(--state-ok));
  }

  .change-letter {
    width: 28px; height: 28px; border-radius: 7px;
    display: flex; align-items: center; justify-content: center;
    font-family: 'DM Mono', monospace; font-size: 11px;
    font-weight: 500; flex-shrink: 0;
    background: rgb(var(--surface-2));
  }
  .change-letter.added { color: rgb(var(--state-ok)); }
  .change-letter.modified { color: rgb(var(--accent)); }
  .change-letter.conflict-c { color: rgb(var(--state-err)); }
</style>
