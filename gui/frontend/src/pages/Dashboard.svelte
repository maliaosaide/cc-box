<script>
  import { onMount, createEventDispatcher } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { GetDashboard, QuickPush, QuickPull, QuickSync, RepairRemoteFromLocal } from '../../wailsjs/go/main/App.js'

  export let syncState = 'idle'
  export let theme = 'dark'
  const dispatch = createEventDispatcher()

  let dashboard = null
  let loading = true
  let actionLoading = ''
  let actionError = ''
  let progress = null
  let currentOpId = null

  onMount(async () => {
    await refresh()
    EventsOn('op:progress', (e) => {
      if (!e.operation || (!e.operation.startsWith('quick-') && e.operation !== 'repair-remote')) return
      if (e.opId === currentOpId || (actionLoading === 'sync' && (e.operation === 'quick-pull' || e.operation === 'quick-push'))) progress = e
    })
    EventsOn('op:complete', async (e) => {
      if (!e || e.opId !== currentOpId) return
      actionLoading = ''; progress = null; currentOpId = null
      if (e.status === 'error') {
        actionError = e.error || '同步失败'
        await refresh()
      } else {
        actionError = ''
        await refresh()
      }
    })
  })

  async function refresh() {
    try {
      dashboard = await GetDashboard()
      if (dashboard && dashboard.conflicts) syncState = 'conflict'
      else syncState = dashboard?.syncStatus || 'idle'
    } catch (e) { syncState = 'error' }
    loading = false
  }

  async function doAction(action) {
    actionLoading = action; actionError = ''; progress = null; syncState = 'syncing'; currentOpId = null
    try {
      currentOpId = action === 'push' ? await QuickPush() : action === 'pull' ? await QuickPull() : await QuickSync()
    } catch (e) {
      actionLoading = ''; syncState = 'error'; actionError = e.message || String(e)
    }
  }

  async function repairRemote() {
    if (!confirm('将使用本机当前配置初始化当前 WebDAV 根路径。请确认根路径正确，且远程没有需要保留的数据。')) return
    actionLoading = 'repair'; actionError = ''; progress = null; syncState = 'syncing'; currentOpId = null
    try {
      currentOpId = await RepairRemoteFromLocal()
    } catch (e) {
      actionLoading = ''; syncState = 'error'; actionError = e.message || String(e)
    }
  }

  function navigateTo(page) { dispatch('navigate', { page }) }

  function statusLabel(state) {
    if (state === 'syncing') return '同步中'
    if (state === 'conflict') return `${data.conflicts} 个冲突`
    if (state === 'pending') return '待同步'
    if (state === 'remote_uninitialized') return '远程未初始化'
    if (state === 'remote_incomplete') return '远程数据不完整'
    if (state === 'key_mismatch') return '密钥不匹配'
    if (state === 'connection_error') return '连接异常'
    if (state === 'local_error') return '本地配置异常'
    if (state === 'error') return '连接异常'
    if (state === 'synced') return '已同步'
    return '未同步'
  }

  $: data = dashboard || {
    syncStatus: 'idle', syncHealth: null, lastSync: null, claudeVersion: '-',
    claudeLatest: true, claudeBinary: null, configStatus: null, conflicts: 0, devices: [],
    recentChanges: [], backups: [], binaries: []
  }
  $: health = data.syncHealth
  $: hasConflicts = data.conflicts !== 0
  $: displaySyncState = hasConflicts ? 'conflict' : (syncState || data.syncStatus || 'idle')
  $: isWarnState = displaySyncState === 'pending' || displaySyncState === 'remote_uninitialized' || displaySyncState === 'idle'
  $: isErrorState = displaySyncState === 'error' || displaySyncState === 'connection_error' || displaySyncState === 'remote_incomplete' || displaySyncState === 'key_mismatch' || displaySyncState === 'local_error'
  $: showRecovery = health && ['remote_uninitialized', 'remote_incomplete', 'key_mismatch', 'connection_error', 'local_error'].includes(displaySyncState)
  $: hasBackups = data.backups && data.backups.length
  $: claudeBinary = data.claudeBinary || {
    platformLabel: '当前平台', localVersion: data.claudeVersion, remoteVersion: '',
    installed: !!data.claudeVersion && data.claudeVersion !== '-', statusLabel: data.claudeLatest ? '已是最新' : '可更新'
  }
  $: configStatus = data.configStatus || {
    ok: true, webdavConfigured: true, passwordAvailable: true, claudeDirExists: true, message: '配置正常'
  }
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
        <button class="theme-switch" on:click={() => dispatch('toggleTheme')} title={theme === 'dark' ? '切换亮色' : '切换暗色'}>
          {#if theme === 'dark'}
            <svg viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.46 4.95l.7.7a1 1 0 001.42-1.41l-.71-.7a1 1 0 00-1.41 1.41zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zM10 15a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.46A1 1 0 106.46 5.05l-.7-.71a1 1 0 00-1.42 1.42l.71.7zM4 11a1 1 0 100-2H3a1 1 0 100 2h1z"/></svg>
            <span>亮色</span>
          {:else}
            <svg viewBox="0 0 20 20" fill="currentColor"><path d="M17.29 13.29A8 8 0 016.71 2.71a8 8 0 1010.58 10.58z"/></svg>
            <span>暗色</span>
          {/if}
        </button>
        <div class="status-pill" class:conflict={displaySyncState === 'conflict'} class:warn={isWarnState} class:err={isErrorState} class:syncing={displaySyncState === 'syncing'}>
          <div class="status-dot" class:ok={displaySyncState === 'synced'} class:warn={isWarnState} class:err={isErrorState || displaySyncState === 'conflict'} class:syncing={displaySyncState === 'syncing'}></div>
          <span class="status-text">{statusLabel(displaySyncState)}</span>
          {#if displaySyncState === 'conflict'}
            <button class="status-link" on:click={() => navigateTo('files')}>解决</button>
          {/if}
        </div>
        <div class="toolbar-divider"></div>
        <div class="action-group">
          <button class="action-btn" disabled={!!actionLoading} on:click={() => doAction('push')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 19V5m-7 7l7-7 7 7"/></svg>
            <span>推送</span>
          </button>
          <button class="action-btn" disabled={!!actionLoading} on:click={() => doAction('pull')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14m-7-7l7 7 7-7"/></svg>
            <span>拉取</span>
          </button>
          <button class="action-btn" disabled={!!actionLoading} on:click={() => doAction('sync')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M1 4v6h6M23 20v-6h-6"/>
              <path d="M20.49 9A9 9 0 005.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 013.51 15"/>
            </svg>
            <span>同步</span>
          </button>
        </div>
      </div>
    </div>

    {#if actionError}
      <div class="error-banner animate-fade-in">{actionError}</div>
    {/if}

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

    {#if showRecovery}
      <div class="recovery-card animate-fade-in">
        <div class="recovery-main">
          <span class="section-label">同步诊断</span>
          <div class="recovery-title">{statusLabel(displaySyncState)}</div>
          <div class="recovery-message">{health.message}</div>
          <div class="recovery-meta">
            {#if health.localHead}<span>本地 HEAD：{health.localHead.slice(0, 12)}</span>{/if}
            {#if health.remoteHead}<span>远程 HEAD：{health.remoteHead.slice(0, 12)}</span>{/if}
          </div>
        </div>
        <div class="recovery-actions">
          <button class="action-btn" disabled={!!actionLoading} on:click={() => navigateTo('settings')}>检查 WebDAV 根路径</button>
          {#if health.canRepair}
            <button class="action-btn primary" disabled={!!actionLoading} on:click={repairRemote}>以本机为准初始化远程</button>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Claude 版本（当前平台） -->
    <div class="card animate-fade-in stagger-1">
      <div class="section-label-row">
        <div>
          <span class="section-label">Claude 版本</span>
          <div class="section-caption">当前平台：{claudeBinary.platformLabel || claudeBinary.platform || '当前平台'}</div>
        </div>
        <button class="link-btn" on:click={() => navigateTo('binaries')}>管理二进制</button>
      </div>
      <div class="item-list">
        <div class="item-row">
          <div class="item-badge accent">C</div>
          <div class="item-main">
            <span class="item-name">当前本地版本</span>
            <span class="item-detail font-mono">{claudeBinary.localVersion || '未检测到'}</span>
          </div>
          <span class="version-tag" class:latest={claudeBinary.status === 'latest'}>{claudeBinary.statusLabel || '未知'}</span>
        </div>
        <div class="item-row">
          <div class="item-badge accent">云</div>
          <div class="item-main">
            <span class="item-name">当前平台云端最高版本</span>
            <span class="item-detail font-mono">{claudeBinary.remoteVersion || '暂无'}</span>
          </div>
        </div>
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

    <!-- 配置状态 -->
    <div class="card animate-fade-in stagger-3">
      <div class="section-label-row">
        <span class="section-label">配置状态</span>
        {#if !configStatus.ok}
          <button class="link-btn" on:click={() => navigateTo('settings')}>去设置</button>
        {/if}
      </div>
      <div class="item-list">
        <div class="item-row">
          <div class="item-badge" class:ok-badge={configStatus.ok} class:warn-badge={!configStatus.ok}>
            {configStatus.ok ? '✓' : '!'}
          </div>
          <div class="item-main">
            <span class="item-name">{configStatus.message || '配置正常'}</span>
            <span class="item-detail">
              WebDAV {configStatus.webdavConfigured ? '已配置' : '未配置'} · 加密密码 {configStatus.passwordAvailable ? '可用' : '不可用'} · Claude 目录 {configStatus.claudeDirExists ? '存在' : '不存在'}
            </span>
          </div>
        </div>
      </div>
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
  .theme-switch {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    border-radius: 8px;
    border: 1px solid rgb(var(--border));
    background: rgb(var(--surface-1));
    color: rgb(var(--text-secondary));
    font-size: 12px;
    font-family: 'DM Mono', monospace;
    cursor: pointer;
    transition: all 0.2s;
  }
  .theme-switch:hover {
    color: rgb(var(--accent));
    border-color: rgba(196,112,78,0.35);
    background: rgba(196,112,78,0.06);
  }
  .theme-switch svg { width: 14px; height: 14px; }
  .status-pill {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 12px;
    border-radius: 8px;
    background: rgb(var(--surface-2));
  }
  .status-pill.conflict, .status-pill.err { background: rgba(184,92,92,0.06); }
  .status-pill.warn { background: rgba(196,165,78,0.08); }
  .status-pill.syncing { background: rgba(91,127,165,0.08); }
  .status-dot { width: 6px; height: 6px; border-radius: 50%; }
  .status-dot.ok { background: rgb(var(--state-ok)); }
  .status-dot.warn { background: rgb(var(--state-warn)); }
  .status-dot.err { background: rgb(var(--state-err)); }
  .status-dot.syncing { background: rgb(var(--state-sync)); }
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
  .action-btn.primary {
    color: rgb(var(--accent));
    background: rgba(196,112,78,0.08);
    border-color: rgba(196,112,78,0.35);
  }
  .action-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  /* 通用 */
  .section-label {
    font-size: 11px; font-weight: 600;
    text-transform: uppercase; letter-spacing: 0.05em;
    color: rgb(var(--text-muted));
  }
  .section-caption {
    margin-top: 2px;
    font-size: 11px;
    color: rgb(var(--text-muted));
    opacity: 0.7;
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
  .error-banner {
    padding: 8px 10px; border-radius: 8px;
    background: rgba(184,92,92,0.08); color: rgb(var(--state-err));
    font-size: 12px;
  }
  .progress-section { margin-top: -4px; }
  .recovery-card {
    display: flex; justify-content: space-between; gap: 12px;
    padding: 12px; border-radius: 10px;
    border: 1px solid rgba(196,165,78,0.25);
    background: rgba(196,165,78,0.06);
  }
  .recovery-main { min-width: 0; flex: 1; }
  .recovery-title { margin-top: 4px; font-size: 14px; font-weight: 600; color: rgb(var(--text-primary)); }
  .recovery-message { margin-top: 4px; font-size: 12px; color: rgb(var(--text-secondary)); line-height: 1.5; }
  .recovery-meta { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 6px; font-size: 11px; color: rgb(var(--text-muted)); font-family: 'DM Mono', monospace; }
  .recovery-actions { display: flex; align-items: flex-start; gap: 6px; flex-shrink: 0; }

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
  .item-badge.ok-badge {
    background: rgba(107,144,128,0.1); color: rgb(var(--state-ok));
  }
  .item-badge.warn-badge {
    background: rgba(196,165,78,0.1); color: rgb(var(--state-warn));
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
</style>
