<script>
  import { onMount, createEventDispatcher } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import {
    GetFileTree, GetFileContent, GetFileDiff,
    GetConflictDetail, ResolveConflict, ExcludeFile,
    BulkSync, SaveMergedConflict
  } from '../../wailsjs/go/main/App.js'
  import TreeNode from '../lib/components/TreeNode.svelte'

  export let syncState = 'idle'
  const dispatch = createEventDispatcher()

  let tree = null
  let loading = true
  let error = ''
  let filter = 'all'
  let selectedPath = ''
  let selectedStatus = ''
  let fileDetail = null
  let diffResult = null
  let conflictDetail = null
  let conflictChoice = ''
  let view = 'content'
  let actionLoading = false
  let progress = null
  let excludeConfirm = ''
  let expandedDirs = new Set([''])

  onMount(async () => {
    await refreshTree()
    EventsOn('op:progress', (e) => {
      if (e.operation && e.operation.startsWith('bulk-')) progress = e
    })
    EventsOn('op:complete', () => {
      actionLoading = false; progress = null; refreshTree()
    })
  })

  async function refreshTree() {
    loading = true; error = ''
    try { tree = await GetFileTree() } catch (e) { error = e.message || String(e) }
    loading = false
  }

  async function handleSelect(e) {
    const { path, status } = e.detail
    selectedPath = path
    selectedStatus = status
    view = 'content'
    diffResult = null; conflictDetail = null; conflictChoice = ''

    if (status === 'conflict') {
      view = 'conflict'
      try { conflictDetail = await GetConflictDetail(path) }
      catch (e) { conflictDetail = { path, local: '加载失败: ' + (e.message || e), remote: '' } }
      return
    }
    try { fileDetail = await GetFileContent(path) } catch (e) { fileDetail = null }
  }

  function handleToggle(e) {
    const { path } = e.detail
    if (expandedDirs.has(path)) expandedDirs.delete(path)
    else expandedDirs.add(path)
    expandedDirs = expandedDirs
  }

  async function showDiff() {
    if (!selectedPath) return
    view = 'diff'
    try { diffResult = await GetFileDiff(selectedPath) }
    catch (e) { diffResult = { path: selectedPath, status: 'error', hunks: [], error: e.message || String(e) } }
  }

  async function resolveConflict(choice) {
    if (!selectedPath || !conflictDetail) return
    conflictChoice = choice
    try {
      if (choice === 'merged') await SaveMergedConflict(selectedPath, conflictDetail.merged || conflictDetail.local)
      else await ResolveConflict(selectedPath, choice)
      await refreshTree()
      selectedPath = ''; conflictDetail = null; view = 'content'
    } catch (e) { error = e.message || String(e) }
  }

  async function confirmExclude() {
    if (!excludeConfirm) return
    try { await ExcludeFile(excludeConfirm); excludeConfirm = ''; await refreshTree() }
    catch (e) { error = e.message || String(e) }
  }

  async function doBulkSync(action) {
    actionLoading = true; progress = null; syncState = 'syncing'
    try { await BulkSync(action) } catch (e) { console.error(action, e) }
  }

  function statusIcon(s) {
    const map = { synced: '✓', modified: 'M', added: 'A', deleted: 'D', conflict: 'C' }
    return map[s] || '·'
  }
  function statusClass(s) {
    const map = { synced: 'st-ok', modified: 'st-mod', added: 'st-add', deleted: 'st-del', conflict: 'st-conflict' }
    return map[s] || ''
  }
  function formatSize(b) {
    if (!b) return '-'
    if (b < 1024) return b + ' B'
    if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
    return (b / 1024 / 1024).toFixed(1) + ' MB'
  }
</script>

<div class="files-page">
  <div class="toolbar animate-fade-in">
    <h1 class="section-title">配置文件</h1>
    <div class="toolbar-right">
      {#if tree}
        <span class="stat">{tree.total} 个文件{#if tree.changed > 0}，<span class="stat-hl">{tree.changed} 个变更</span>{/if}</span>
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

  {#if error}
    <div class="error-bar animate-fade-in">
      <span>{error}</span>
      <button class="link-btn" on:click={() => { error = '' }}>关闭</button>
    </div>
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
          <button class="filter-btn" class:active={filter === 'all'} on:click={() => filter = 'all'}>全部</button>
          <button class="filter-btn" class:active={filter === 'changed'} on:click={() => filter = 'changed'}>已变更</button>
          <button class="filter-btn" class:active={filter === 'conflict'} on:click={() => filter = 'conflict'}>
            冲突{#if tree.conflicts > 0} ({tree.conflicts}){/if}
          </button>
        </div>
        <div class="tree-list">
          {#if tree.root && tree.root.children && tree.root.children.length > 0}
            {#each tree.root.children as node}
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
              <span class="conflict-label">冲突 — 请选择版本</span>
            </div>
            <div class="conflict-panels">
              <div class="conflict-col">
                <div class="conflict-col-header">
                  <button class="choice-btn" class:chosen={conflictChoice === 'local'} on:click={() => conflictChoice = 'local'}>本地版本</button>
                </div>
                <pre class="conflict-code">{conflictDetail.local || '(空)'}</pre>
              </div>
              <div class="conflict-col">
                <div class="conflict-col-header">
                  <button class="choice-btn" class:chosen={conflictChoice === 'remote'} on:click={() => conflictChoice = 'remote'}>远程版本</button>
                </div>
                <pre class="conflict-code">{conflictDetail.remote || '(空)'}</pre>
              </div>
            </div>
            <div class="conflict-actions">
              <button class="btn-ghost" on:click={() => { selectedPath = ''; conflictDetail = null }}>取消</button>
              <button class="btn-primary" disabled={!conflictChoice} on:click={() => resolveConflict(conflictChoice)}>
                使用{conflictChoice === 'local' ? '本地' : '远程'}版本
              </button>
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
    <div class="modal-overlay" on:click={() => { excludeConfirm = '' }}>
      <div class="modal-card" on:click|stopPropagation>
        <p class="text-sm text-txt-primary">确认排除文件？</p>
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

  .error-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(184,92,92,0.08); border: 1px solid rgba(184,92,92,0.15);
    font-size: 12px; color: rgb(var(--state-err));
  }

  .content-layout {
    display: flex; gap: 1px; flex: 1; min-height: 0;
    background: rgb(var(--border)); border-radius: 8px; overflow: hidden;
  }

  .tree-panel {
    width: 260px; min-width: 260px;
    background: rgb(var(--surface-1));
    display: flex; flex-direction: column;
  }
  .filter-bar { display: flex; gap: 2px; padding: 8px 8px 4px; border-bottom: 1px solid rgb(var(--border)); }
  .filter-btn {
    flex: 1; padding: 4px 8px; border-radius: 4px;
    font-size: 11px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: transparent; border: none; cursor: pointer;
    transition: all 0.2s;
  }
  .filter-btn:hover { color: rgb(var(--text-secondary)); }
  .filter-btn.active { background: rgb(var(--surface-2)); color: rgb(var(--accent)); }
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
  .st-conflict { color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }

  .file-content {
    flex: 1; overflow: auto; padding: 12px 14px; margin: 0;
    font-size: 12px; font-family: 'DM Mono', monospace;
    line-height: 1.6; color: rgb(var(--text-primary));
    background: transparent; white-space: pre-wrap; word-break: break-all;
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
  .conflict-col-header { padding: 6px 10px; border-bottom: 1px solid rgb(var(--border)); background: rgb(var(--surface-1)); }
  .choice-btn {
    font-size: 11px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: none; border: none; cursor: pointer;
    padding: 2px 8px; border-radius: 3px; transition: all 0.2s;
  }
  .choice-btn:hover { color: rgb(var(--text-secondary)); }
  .choice-btn.chosen { background: rgba(196,112,78,0.1); color: rgb(var(--accent)); }
  .conflict-code {
    flex: 1; overflow: auto; padding: 10px;
    font-size: 12px; font-family: 'DM Mono', monospace;
    line-height: 1.6; color: rgb(var(--text-primary));
    background: transparent; white-space: pre-wrap; margin: 0;
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
