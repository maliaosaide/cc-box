<script>
  import { onMount, createEventDispatcher } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import {
    GetFileTreeLocal, GetFileTree, GetFileContent, GetFileDiff,
    GetConflictDetail, ResolveConflict, ExcludeFile,
    BulkSync, SaveMergedConflict
  } from '../../wailsjs/go/main/App.js'
  import { formatSize } from '../lib/utils.js'
  import TreeNode from '../lib/components/TreeNode.svelte'

  export let syncState = 'idle'
  export let active = false
  export let refreshToken = 0
  const dispatch = createEventDispatcher()

  let tree = null
  let loading = true
  let error = ''
  let msg = ''
  let msgTimer = null
  let remoteError = ''
  let filter = 'all'
  let selectedPath = ''
  let selectedStatus = ''
  let fileDetail = null
  let diffResult = null
  let conflictDetail = null
  let failureDetail = null
  let conflictExpanded = false
  let resolveLoading = false
  let conflictChoice = ''
  let conflictViewMode = 'side'
  let view = 'content'
  let actionLoading = false
  let progress = null
  let excludeConfirm = ''
  let expandedDirs = new Set([''])
  let dirty = false
  let detailRequestId = 0
  let diffRequestId = 0
  let lastRefreshToken = 0

  $: if (active && refreshToken !== lastRefreshToken) {
    lastRefreshToken = refreshToken
    refreshTree()
  }

  $: if (active && dirty) {
    dirty = false
    refreshTree()
  }

  onMount(async () => {
    await refreshTree()
    EventsOn('op:progress', (e) => {
      if (e.operation && e.operation.startsWith('bulk-')) progress = e
    })
    EventsOn('op:complete', (e) => {
      if (!affectsFiles(e?.operation)) return
      actionLoading = false; progress = null
      if (e.status === 'error') error = e.error || '同步失败'
      if (active) refreshTree()
      else dirty = true
    })
  })

  async function refreshTree() {
    loading = !tree; error = ''; remoteError = ''
    try {
      tree = await GetFileTreeLocal()
      loading = false
    } catch (e) {
      if (!tree) error = e.message || String(e)
      loading = false
      return
    }
    try {
      const remoteTree = await GetFileTree()
      tree = remoteTree
    } catch (e) {
      remoteError = e.message || String(e)
      syncState = 'connection_error'
      if (!tree) error = remoteError
    }
    loading = false
  }

  function affectsFiles(operation) {
    return operation && (operation.startsWith('bulk-') || operation.startsWith('quick-') || operation === 'repair-remote')
  }

  async function handleSelect(e) {
    const { path, status, error: failureError, fullPath } = e.detail
    const requestId = ++detailRequestId
    diffRequestId += 1
    selectedPath = path
    selectedStatus = status
    view = 'content'
    fileDetail = null; diffResult = null; conflictDetail = null; failureDetail = null; conflictChoice = ''

    if (status === 'failed') {
      view = 'failed'
      failureDetail = { path, fullPath, error: failureError }
      return
    }

    if (status === 'conflict') {
      view = 'conflict'
      try {
        const detail = await GetConflictDetail(path)
        if (requestId === detailRequestId && selectedPath === path) conflictDetail = detail
      }
      catch (e) {
        if (requestId === detailRequestId && selectedPath === path) conflictDetail = { path, local: '加载失败: ' + (e.message || e), remote: '' }
      }
      return
    }
    try {
      const detail = await GetFileContent(path)
      if (requestId === detailRequestId && selectedPath === path) fileDetail = detail
    } catch (e) {
      if (requestId === detailRequestId && selectedPath === path) fileDetail = null
    }
  }

  function handleToggle(e) {
    const { path } = e.detail
    if (expandedDirs.has(path)) expandedDirs.delete(path)
    else expandedDirs.add(path)
    expandedDirs = expandedDirs
  }

  async function showDiff() {
    if (!selectedPath) return
    const requestPath = selectedPath
    const requestId = ++diffRequestId
    view = 'diff'
    diffResult = null
    try {
      const result = await GetFileDiff(requestPath)
      if (requestId === diffRequestId && selectedPath === requestPath && view === 'diff') diffResult = result
    }
    catch (e) {
      if (requestId === diffRequestId && selectedPath === requestPath && view === 'diff') diffResult = { path: requestPath, status: 'error', hunks: [], error: e.message || String(e) }
    }
  }

  function conflictTruncated(text) {
    if (!text) return ''
    const max = conflictExpanded ? 500000 : 2000
    if (text.length <= max) return text
    return text.slice(0, max) + '\n\n... (文件过长，已截断，点击下方"显示全文"查看完整内容)'
  }

  function showMsg(text) {
    clearTimeout(msgTimer)
    msg = text
    msgTimer = setTimeout(() => { msg = '' }, 4000)
  }

  async function resolveConflict(choice) {
    if (!selectedPath || !conflictDetail || resolveLoading) return
    resolveLoading = true
    try {
      if (choice === 'merged') await SaveMergedConflict(selectedPath, conflictDetail.merged || conflictDetail.local)
      else await ResolveConflict(selectedPath, choice)
      await refreshTree()
      showMsg('已解决冲突')
      selectedPath = ''; conflictDetail = null; failureDetail = null; view = 'content'; conflictChoice = ''
    } catch (e) { error = e.message || String(e) }
    resolveLoading = false
  }

  async function confirmExclude() {
    if (!excludeConfirm) return
    try { await ExcludeFile(excludeConfirm); excludeConfirm = ''; await refreshTree(); showMsg('已添加到排除规则') }
    catch (e) { error = e.message || String(e) }
  }

  function closeExcludeModal(e) {
    if (e.target === e.currentTarget) excludeConfirm = ''
  }

  async function doBulkSync(action) {
    actionLoading = true; progress = null; syncState = 'syncing'; error = ''
    try { await BulkSync(action) }
    catch (e) {
      actionLoading = false; progress = null; syncState = 'error'; error = e.message || String(e)
    }
  }

  function statusIcon(s) {
    const map = { synced: '✓', modified: 'M', added: 'A', deleted: 'D', conflict: 'C', failed: '!', checking: '…' }
    return map[s] || '·'
  }
  function statusClass(s) {
    const map = { synced: 'st-ok', modified: 'st-mod', added: 'st-add', deleted: 'st-del', conflict: 'st-conflict', failed: 'st-failed', checking: 'st-checking' }
    return map[s] || ''
  }
  function isChangedStatus(status) {
    return ['modified', 'added', 'deleted', 'conflict'].includes(status)
  }

  function matchesFilter(node, activeFilter) {
    if (activeFilter === 'all') return true
    if (node.isDir) return (node.children || []).some(child => matchesFilter(child, activeFilter))
    if (activeFilter === 'changed') return isChangedStatus(node.status)
    if (activeFilter === 'conflict') return node.status === 'conflict'
    if (activeFilter === 'failed') return node.status === 'failed'
    return true
  }

  $: mergedContent = (conflictDetail && conflictDetail.local && conflictDetail.remote)
    ? `<<<<<<< 本地版本\n${conflictDetail.local}\n=======\n${conflictDetail.remote}\n>>>>>>> 远程版本`
    : ''

  $: rootChildren = tree?.root?.children ? tree.root.children.filter(node => matchesFilter(node, filter)) : []
</script>

<div class="files-page">
  <div class="toolbar animate-fade-in">
    <h1 class="section-title">配置文件</h1>
    <div class="toolbar-right">
      {#if tree}
        <span class="stat">{tree.total} 个文件{#if tree.checking}，正在检查状态...{:else if tree.failed > 0}，<span class="stat-err">{tree.failed} 个失败</span>{:else if tree.changed > 0}，<span class="stat-hl">{tree.changed} 个变更</span>{/if}</span>
        <div class="toolbar-divider"></div>
      {/if}
      <div class="action-group">
        <button class="action-btn" disabled={actionLoading} on:click={() => doBulkSync('push')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 19V5m-7 7l7-7 7 7"/></svg>
          <span>推送所有</span>
        </button>
        <button class="action-btn" disabled={actionLoading} on:click={() => doBulkSync('pull')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14m-7-7l7 7 7-7"/></svg>
          <span>拉取所有</span>
        </button>
      </div>
    </div>
  </div>

  {#if msg}
    <div class="msg-bar animate-fade-in">
      <span>{msg}</span>
      <button class="link-btn" on:click={() => { clearTimeout(msgTimer); msg = '' }}>关闭</button>
    </div>
  {/if}

  {#if error}
    <div class="error-bar animate-fade-in">
      <span>{error}</span>
      <button class="link-btn" on:click={() => { error = '' }}>关闭</button>
    </div>
  {/if}

  {#if remoteError}
    <div class="error-bar animate-fade-in">
      <span>远程刷新失败：{remoteError}</span>
      <button class="link-btn" on:click={() => { remoteError = '' }}>关闭</button>
    </div>
  {/if}

  <div class="sync-notice animate-fade-in">
    <span>未被排除规则命中的配置文件都会同步，包括大文件和敏感配置；不想上传的文件请添加到排除规则。</span>
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

  {#if loading}
    <div class="flex items-center justify-center h-64">
      <div class="loading-dot animate-gentle-pulse"></div>
    </div>
  {:else if !tree}
    <div class="card flex items-center justify-center py-20">
      <p class="text-txt-muted text-sm">无法加载文件树</p>
    </div>
  {:else}
    <div class="content-layout">
      <div class="tree-panel">
        <div class="filter-bar">
          <button class="filter-btn" class:active={filter === 'all'} on:click={() => filter = 'all'}>
            <span class="filter-label">文件</span>
            <span class="filter-count">{tree.total}</span>
          </button>
          <button class="filter-btn" class:active={filter === 'changed'} on:click={() => filter = 'changed'}>
            <span class="filter-label">已变更</span>
            <span class="filter-count">{tree.changed}</span>
          </button>
          <button class="filter-btn filter-conflict" class:active={filter === 'conflict'} on:click={() => filter = 'conflict'}>
            <span class="filter-label">冲突</span>
            <span class="filter-count" class:has-conflict={tree.conflicts > 0}>{tree.conflicts}</span>
          </button>
          <button class="filter-btn filter-failed" class:active={filter === 'failed'} on:click={() => filter = 'failed'}>
            <span class="filter-label">错误</span>
            <span class="filter-count" class:has-conflict={tree.failed > 0}>{tree.failed}</span>
          </button>
        </div>
        <div class="tree-list">
          {#if rootChildren.length > 0}
            {#each rootChildren as node}
              <TreeNode {node} {selectedPath} {expandedDirs} {filter}
                on:toggle={handleToggle} on:select={handleSelect} />
            {/each}
          {:else}
            <div class="empty-compact">{filter === 'all' ? '暂无文件' : '没有匹配的文件'}</div>
          {/if}
        </div>
      </div>

      <div class="detail-panel">
        {#if !selectedPath}
          <div class="empty-detail">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="40" height="40">
              <path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
            </svg>
            <p class="text-txt-muted text-sm">选择文件查看详情</p>
          </div>
        {:else if view === 'conflict' && conflictDetail}
          <div class="conflict-view">
            <div class="detail-header">
              <div class="detail-title-row">
                <span class="status-badge st-conflict">C</span>
                <span class="font-mono text-sm text-txt-primary">{selectedPath}</span>
              </div>
              <div class="detail-actions">
                <span class="conflict-label">
                  冲突 — {conflictDetail.recommended === 'local' ? '本地较新' : conflictDetail.recommended === 'remote' ? '远程较新' : '请比较两边'}
                </span>
                <button class="view-toggle-btn" class:active={conflictViewMode === 'side'} on:click={() => conflictViewMode = 'side'}>并排</button>
                <button class="view-toggle-btn" class:active={conflictViewMode === 'inline'} on:click={() => conflictViewMode = 'inline'}>合并</button>
              </div>
            </div>
            {#if conflictViewMode === 'side'}
              <div class="conflict-panels">
                <div class="conflict-col">
                  <div class="conflict-col-header">
                    <button class="choice-btn" class:chosen={conflictChoice === 'local'} on:click={() => conflictChoice = 'local'}>
                      本地版本{#if conflictDetail.recommended === 'local'}<span class="newer-tag">较新</span>{/if}
                    </button>
                    <span class="version-time">{conflictDetail.localExists ? (conflictDetail.localModified || '时间未知') : '本地已删除'}</span>
                  </div>
                  <pre class="conflict-code">{conflictDetail.localExists ? conflictTruncated(conflictDetail.local || '(空)') : '(本地已删除)'}</pre>
                </div>
                <div class="conflict-col">
                  <div class="conflict-col-header">
                    <button class="choice-btn" class:chosen={conflictChoice === 'remote'} on:click={() => conflictChoice = 'remote'}>
                      远程版本{#if conflictDetail.recommended === 'remote'}<span class="newer-tag">较新</span>{/if}
                    </button>
                    <span class="version-time">{conflictDetail.remoteExists ? (conflictDetail.remoteModified || '时间未知') : '远程已删除'}</span>
                  </div>
                  <pre class="conflict-code">{conflictDetail.remoteExists ? conflictTruncated(conflictDetail.remote || '(空)') : '(远程已删除)'}</pre>
                </div>
              </div>
            {:else}
              <div class="conflict-merged-view">
                <pre class="conflict-code merged-code">{conflictTruncated(mergedContent)}</pre>
              </div>
            {/if}
            <div class="conflict-actions">
              {#if (conflictDetail.local && conflictDetail.local.length > 2000) || (conflictDetail.remote && conflictDetail.remote.length > 2000)}
                <button class="btn-ghost" on:click={() => conflictExpanded = !conflictExpanded}>
                  {conflictExpanded ? '收起全文' : '显示全文'}
                </button>
              {/if}
              <button class="btn-ghost" on:click={() => { selectedPath = ''; conflictDetail = null; conflictExpanded = false; conflictViewMode = 'side' }}>取消</button>
              <button class="btn-primary" disabled={!conflictChoice || resolveLoading} on:click={() => resolveConflict(conflictChoice)}>
                {resolveLoading ? '处理中...' : conflictChoice ? `以${conflictChoice === 'local' ? '本地' : '远程'}为准` : '请选择版本'}
              </button>
            </div>
          </div>
        {:else if view === 'failed' && failureDetail}
          <div class="failed-view">
            <div class="detail-header">
              <div class="detail-title-row">
                <span class="status-badge st-failed">!</span>
                <span class="font-mono text-sm text-txt-primary">{selectedPath}</span>
              </div>
              <span class="failed-label">失败文件</span>
            </div>
            <div class="failed-body">
              <div class="failed-row">
                <span>来源路径</span>
                <code>{failureDetail.fullPath || selectedPath}</code>
              </div>
              <div class="failed-row">
                <span>失败原因</span>
                <code>{failureDetail.error || '无法读取文件'}</code>
              </div>
              <p>修复权限、锁定或符号链接问题后，可重新上传。未加入排除规则的文件必须成功上传。</p>
              <div class="failed-actions">
                <button class="btn-ghost" on:click={() => excludeConfirm = selectedPath}>加入排除规则</button>
                <button class="btn-primary" disabled={actionLoading} on:click={() => doBulkSync('push')}>重新上传失败文件</button>
              </div>
            </div>
          </div>
        {:else if view === 'diff' && diffResult}
          <div class="diff-view">
            <div class="detail-header">
              <div class="detail-title-row">
                <span class="status-badge {statusClass(diffResult.status)}">{statusIcon(diffResult.status)}</span>
                <span class="font-mono text-sm text-txt-primary">{selectedPath}</span>
              </div>
              <div class="diff-meta">
                <span class="diff-label">{diffResult.status === 'modified' ? '已修改' : diffResult.status === 'added' ? '新增' : diffResult.status === 'deleted' ? '已删除' : '一致'}</span>
                <button class="link-btn" on:click={() => view = 'content'}>返回内容</button>
              </div>
            </div>
            {#if diffResult.hunks && diffResult.hunks.length > 0}
              <div class="diff-content">
                {#each diffResult.hunks as hunk}
                  <div class="diff-hunk-header">@@ -{hunk.oldStart},{hunk.oldCount} +{hunk.newStart},{hunk.newCount} @@</div>
                  {#each hunk.lines as line}
                    <div class="diff-line" class:diff-add={line[0] === '+'} class:diff-del={line[0] === '-'}>
                      <span class="diff-prefix">{line[0]}</span><span class="diff-text">{line.slice(1)}</span>
                    </div>
                  {/each}
                {/each}
              </div>
            {:else if diffResult.status === 'synced'}
              <div class="empty-compact">文件内容一致，无差异</div>
            {:else}
              <pre class="diff-raw">{diffResult.error || '无法计算差异'}</pre>
            {/if}
          </div>
        {:else}
          <div class="content-view">
            <div class="detail-header">
              <div class="detail-title-row">
                <span class="status-badge {statusClass(selectedStatus)}">{statusIcon(selectedStatus)}</span>
                <span class="font-mono text-sm text-txt-primary">{selectedPath}</span>
              </div>
              <div class="detail-actions">
                {#if fileDetail}<span class="text-xs text-txt-muted">{formatSize(fileDetail.size)}</span>{/if}
                <button class="link-btn" on:click={showDiff}>查看 Diff</button>
                <button class="link-btn" style="color: rgb(var(--state-err))" on:click={() => excludeConfirm = selectedPath}>排除</button>
              </div>
            </div>
            {#if fileDetail && fileDetail.content}
              <pre class="file-content">{fileDetail.content}</pre>
            {:else if fileDetail}
              <div class="empty-compact">二进制文件，不支持预览</div>
            {:else}
              <div class="empty-compact">无法加载文件内容</div>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#if excludeConfirm}
    <div class="modal-overlay" role="presentation" on:click={closeExcludeModal}>
      <div class="modal-card" role="dialog" aria-modal="true" aria-labelledby="exclude-dialog-title">
        <p id="exclude-dialog-title" class="text-sm text-txt-primary">确认排除文件？</p>
        <p class="text-xs text-txt-muted font-mono mt-1">{excludeConfirm}</p>
        <p class="text-xs text-txt-muted mt-2">排除后将不再参与同步</p>
        <div class="modal-actions">
          <button class="btn-ghost" on:click={() => { excludeConfirm = '' }}>取消</button>
          <button class="btn-primary" on:click={confirmExclude}>确认排除</button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .files-page { display: flex; flex-direction: column; gap: 12px; height: 100%; }

  .toolbar { display: flex; align-items: center; justify-content: space-between; flex-shrink: 0; }
  .toolbar-right { display: flex; align-items: center; gap: 10px; }
  .stat { font-size: 11px; font-family: 'DM Mono', monospace; color: rgb(var(--text-muted)); }
  .stat-hl { color: rgb(var(--accent)); }
  .stat-err { color: rgb(var(--state-err)); }
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
  .action-btn:hover { border-color: rgba(196,112,78,0.4); color: rgb(var(--accent)); background: rgba(196,112,78,0.05); }
  .action-btn:disabled { opacity: 0.4; cursor: not-allowed; }

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
  .sync-notice {
    padding: 8px 12px; border-radius: 6px;
    background: rgba(196,112,78,0.06); border: 1px solid rgba(196,112,78,0.14);
    font-size: 12px; color: rgb(var(--text-muted));
  }

  .content-layout {
    display: flex; gap: 1px; flex: 1; min-height: 0;
    background: rgb(var(--border)); border-radius: 8px; overflow: hidden;
  }

  .tree-panel {
    width: 340px; min-width: 340px;
    background: rgb(var(--surface-1));
    display: flex; flex-direction: column;
  }
  .filter-bar {
    display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px;
    padding: 9px 10px; border-bottom: 1px solid rgb(var(--border));
    background: linear-gradient(180deg, rgba(255,255,255,0.02), transparent);
  }
  .filter-btn {
    min-height: 44px; padding: 7px 6px; border-radius: 8px;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: rgba(var(--surface-2), 0.5);
    border: 1px solid rgba(var(--border), 0.9); cursor: pointer;
    transition: all 0.2s ease-out;
    display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px;
  }
  .filter-btn:hover { color: rgb(var(--text-secondary)); background: rgb(var(--surface-2)); transform: translateY(-1px); }
  .filter-btn.active {
    background: rgba(196,112,78,0.12); color: rgb(var(--accent));
    border-color: rgba(196,112,78,0.35); box-shadow: inset 0 0 0 1px rgba(196,112,78,0.1);
  }
  .filter-btn.filter-conflict.active, .filter-btn.filter-failed.active {
    background: rgba(184,92,92,0.1); color: rgb(var(--state-err));
    border-color: rgba(184,92,92,0.35);
  }
  .filter-label {
    font-size: 10px; line-height: 1; font-weight: 600; letter-spacing: 0.02em; white-space: nowrap;
  }
  .filter-count {
    font-size: 14px; line-height: 1.1; font-weight: 700; font-family: 'DM Mono', monospace;
    color: rgb(var(--text-secondary));
  }
  .filter-btn.active .filter-count { color: rgb(var(--accent)); }
  .filter-btn.filter-conflict.active .filter-count, .filter-btn.filter-failed.active .filter-count { color: rgb(var(--state-err)); }
  .filter-count.has-conflict { color: rgb(var(--state-err)); }
  .tree-list { flex: 1; overflow-y: auto; padding: 4px 0; }

  .detail-panel {
    flex: 1; background: rgb(var(--surface-0));
    display: flex; flex-direction: column; min-width: 0; overflow: hidden;
  }
  .empty-detail {
    flex: 1; display: flex; flex-direction: column;
    align-items: center; justify-content: center; gap: 8px;
    color: rgb(var(--text-muted)); opacity: 0.4;
  }
  .detail-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 10px 14px; border-bottom: 1px solid rgb(var(--border)); flex-shrink: 0;
  }
  .detail-title-row { display: flex; align-items: center; gap: 8px; }
  .detail-actions { display: flex; align-items: center; gap: 10px; }

  .status-badge {
    width: 18px; height: 18px; border-radius: 4px;
    display: inline-flex; align-items: center; justify-content: center;
    font-size: 10px; font-family: 'DM Mono', monospace;
    font-weight: 500; flex-shrink: 0;
    background: rgb(var(--surface-2)); color: rgb(var(--text-muted));
  }
  .st-ok { color: rgb(var(--state-ok)); }
  .st-mod { color: rgb(var(--accent)); }
  .st-add { color: rgb(var(--state-ok)); }
  .st-del { color: rgb(var(--text-muted)); opacity: 0.5; }
  .st-checking { color: rgb(var(--text-muted)); }
  .st-failed { color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }
  .st-conflict { color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }

  .failed-view { display: flex; flex-direction: column; flex: 1; }
  .failed-label { font-size: 11px; color: rgb(var(--state-err)); font-family: 'DM Mono', monospace; }
  .failed-body { padding: 14px; display: flex; flex-direction: column; gap: 12px; font-size: 12px; color: rgb(var(--text-secondary)); }
  .failed-row { display: flex; flex-direction: column; gap: 4px; }
  .failed-row span { color: rgb(var(--text-muted)); }
  .failed-row code { color: rgb(var(--text-primary)); font-family: 'DM Mono', monospace; word-break: break-all; }
  .failed-actions { display: flex; gap: 8px; }

  .content-view {
    display: flex; flex-direction: column; flex: 1; min-height: 0;
  }
  .file-content {
    flex: 1; overflow: auto; padding: 12px 14px; margin: 0;
    font-size: 12px; font-family: 'DM Mono', monospace;
    line-height: 1.6; color: rgb(var(--text-primary));
    background: transparent; white-space: pre-wrap; word-break: break-all;
  }

  .diff-view {
    display: flex; flex-direction: column; flex: 1; min-height: 0;
  }
  .diff-meta { display: flex; align-items: center; gap: 8px; }
  .diff-label { font-size: 11px; font-family: 'DM Mono', monospace; color: rgb(var(--accent)); }
  .diff-content { flex: 1; overflow: auto; font-family: 'DM Mono', monospace; font-size: 12px; }
  .diff-hunk-header { padding: 4px 14px; font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.6; background: rgba(196,112,78,0.03); }
  .diff-line { display: flex; min-height: 20px; line-height: 20px; }
  .diff-prefix { width: 20px; text-align: center; flex-shrink: 0; user-select: none; opacity: 0.4; }
  .diff-text { flex: 1; white-space: pre; padding-right: 14px; }
  .diff-add { background: rgba(107,144,128,0.06); }
  .diff-add .diff-prefix { color: rgb(var(--state-ok)); }
  .diff-del { background: rgba(184,92,92,0.06); }
  .diff-del .diff-prefix { color: rgb(var(--state-err)); }
  .diff-raw { padding: 12px 14px; font-size: 12px; font-family: 'DM Mono', monospace; color: rgb(var(--text-muted)); }

  .conflict-view { display: flex; flex-direction: column; flex: 1; }
  .conflict-label { font-size: 11px; color: rgb(var(--state-err)); font-family: 'DM Mono', monospace; }
  .conflict-panels { display: flex; flex: 1; min-height: 0; overflow: hidden; }
  .conflict-col { flex: 1; display: flex; flex-direction: column; border-right: 1px solid rgb(var(--border)); min-width: 0; }
  .conflict-col:last-child { border-right: none; }
  .conflict-col-header { padding: 6px 10px; border-bottom: 1px solid rgb(var(--border)); background: rgb(var(--surface-1)); display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .choice-btn {
    font-size: 11px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: none; border: none; cursor: pointer;
    padding: 2px 8px; border-radius: 3px; transition: all 0.2s;
  }
  .choice-btn:hover { color: rgb(var(--text-secondary)); }
  .choice-btn.chosen { background: rgba(196,112,78,0.1); color: rgb(var(--accent)); }
  .newer-tag { margin-left: 6px; padding: 1px 5px; border-radius: 3px; background: rgba(107,144,128,0.12); color: rgb(var(--state-ok)); font-size: 10px; }
  .version-time { font-size: 10px; color: rgb(var(--text-muted)); font-family: 'DM Mono', monospace; white-space: nowrap; }
  .conflict-code {
    flex: 1; overflow: auto; padding: 10px;
    font-size: 12px; font-family: 'DM Mono', monospace;
    line-height: 1.6; color: rgb(var(--text-primary));
    background: transparent; white-space: pre-wrap; margin: 0;
  }
  .view-toggle-btn {
    font-size: 10px; font-weight: 600;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: rgb(var(--surface-2));
    border: 1px solid rgb(var(--border));
    padding: 3px 10px; border-radius: 4px; cursor: pointer;
    transition: all 0.2s;
  }
  .view-toggle-btn.active {
    background: rgba(196,112,78,0.12); color: rgb(var(--accent));
    border-color: rgba(196,112,78,0.35);
  }
  .conflict-merged-view {
    flex: 1; min-height: 0; display: flex; flex-direction: column;
  }
  .merged-code {
    white-space: pre-wrap; word-break: break-all;
  }
  .conflict-actions {
    display: flex; justify-content: flex-end; gap: 8px;
    padding: 8px 14px; border-top: 1px solid rgb(var(--border));
    background: rgb(var(--surface-1));
  }

  .modal-overlay {
    position: fixed; inset: 0; background: rgba(0,0,0,0.4);
    display: flex; align-items: center; justify-content: center; z-index: 100;
  }
  .modal-card {
    background: rgb(var(--surface-1)); border: 1px solid rgb(var(--border));
    border-radius: 10px; padding: 20px; max-width: 360px; width: 90%;
  }
  .modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }

  .link-btn {
    font-size: 11px; color: rgb(var(--text-muted));
    background: none; border: none; cursor: pointer; transition: color 0.2s;
  }
  .link-btn:hover { color: rgb(var(--accent)); }
  .btn-ghost {
    font-size: 12px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-secondary)); background: rgb(var(--surface-2));
    border: 1px solid rgb(var(--border));
    padding: 6px 14px; border-radius: 6px; cursor: pointer; transition: all 0.2s;
  }
  .btn-ghost:hover { border-color: rgb(var(--accent)); color: rgb(var(--accent)); }
  .btn-primary {
    font-size: 12px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: #fff; background: linear-gradient(135deg, rgb(var(--accent)), rgba(196,112,78,0.8));
    border: none; padding: 6px 14px; border-radius: 6px; cursor: pointer; transition: opacity 0.2s;
  }
  .btn-primary:hover { opacity: 0.9; }
  .btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }
  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
  .progress-section { margin-top: -4px; }
</style>
