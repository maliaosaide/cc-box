<script>
  import { createEventDispatcher } from 'svelte'

  export let node
  export let selectedPath = ''
  export let expandedDirs = new Set()
  export let filter = 'all'

  const dispatch = createEventDispatcher()

  function statusIcon(status) {
    switch (status) {
      case 'synced': return '✓'
      case 'modified': return 'M'
      case 'added': return 'A'
      case 'deleted': return 'D'
      case 'conflict': return 'C'
      default: return '·'
    }
  }

  function statusClass(status) {
    switch (status) {
      case 'synced': return 'st-ok'
      case 'modified': return 'st-mod'
      case 'added': return 'st-add'
      case 'deleted': return 'st-del'
      case 'conflict': return 'st-conflict'
      default: return ''
    }
  }

  function formatSize(bytes) {
    if (!bytes) return '-'
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  }

  function toggleDir(path) {
    dispatch('toggle', { path })
  }

  function selectFile(path, status) {
    dispatch('select', { path, status })
  }

  $: isExpanded = expandedDirs.has(node.path)
  $: filteredChildren = filter === 'all'
    ? (node.children || [])
    : (node.children || []).filter(c => {
        if (c.isDir) return hasMatchingChild(c, filter)
        if (filter === 'changed') return c.status !== 'synced'
        if (filter === 'conflict') return c.status === 'conflict'
        return true
      })

  function hasMatchingChild(n, f) {
    if (!n.children) return false
    for (const c of n.children) {
      if (c.isDir && hasMatchingChild(c, f)) return true
      if (f === 'changed' && c.status !== 'synced') return true
      if (f === 'conflict' && c.status === 'conflict') return true
    }
    return false
  }

  function aggregateStatus(n) {
    let hasConflict = false, hasModified = false, hasAdded = false, hasDeleted = false, hasSynced = false
    function walk(node) {
      if (!node.children) return
      for (const c of node.children) {
        if (c.isDir) { walk(c); continue }
        switch (c.status) {
          case 'conflict': hasConflict = true; break
          case 'modified': hasModified = true; break
          case 'added': hasAdded = true; break
          case 'deleted': hasDeleted = true; break
          default: hasSynced = true
        }
      }
    }
    walk(n)
    if (hasConflict) return 'conflict'
    if (hasModified || hasAdded) return 'modified'
    if (hasDeleted) return 'deleted'
    return 'synced'
  }
</script>

{#if node.isDir}
  <button class="tree-dir" type="button" on:click={() => toggleDir(node.path)}>
    <span class="dir-arrow" class:open={isExpanded}>▶</span>
    <span class="dir-icon">📁</span>
    <span class="tree-name">{node.name}</span>
    <span class="status-badge {statusClass(aggregateStatus(node))}">{statusIcon(aggregateStatus(node))}</span>
  </button>
  {#if isExpanded && filteredChildren.length > 0}
    <div class="tree-children">
      {#each filteredChildren as child}
        <svelte:self node={child} {selectedPath} {expandedDirs} {filter}
          on:toggle={(e) => dispatch('toggle', e.detail)}
          on:select={(e) => dispatch('select', e.detail)} />
      {/each}
    </div>
  {/if}
{:else}
  <button class="tree-file" class:selected={selectedPath === node.path} type="button"
       on:click={() => selectFile(node.path, node.status)}>
    <span class="status-badge {statusClass(node.status)}">{statusIcon(node.status)}</span>
    <span class="tree-name">{node.name}</span>
    <span class="tree-meta">{formatSize(node.size)}</span>
  </button>
{/if}

<style>
  .tree-dir, .tree-file {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 4px 10px; border: 0; background: transparent;
    color: inherit; font: inherit; text-align: left; cursor: pointer;
    transition: background 0.15s;
  }
  .tree-dir:hover, .tree-file:hover { background: rgba(196,112,78,0.04); }
  .tree-file.selected { background: rgba(196,112,78,0.08); }
  .tree-name {
    flex: 1; font-size: 12px; color: rgb(var(--text-primary));
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .tree-meta {
    font-size: 10px; font-family: 'DM Mono', monospace;
    color: rgb(var(--text-muted)); opacity: 0.6;
  }
  .tree-children { padding-left: 14px; }

  .dir-arrow {
    font-size: 8px; color: rgb(var(--text-muted));
    transition: transform 0.15s;
  }
  .dir-arrow.open { transform: rotate(90deg); }
  .dir-icon { font-size: 12px; }

  .status-badge {
    width: 18px; height: 18px; border-radius: 4px;
    display: inline-flex; align-items: center; justify-content: center;
    font-size: 10px; font-family: 'DM Mono', monospace;
    font-weight: 500; flex-shrink: 0;
    background: rgb(var(--surface-2));
    color: rgb(var(--text-muted));
  }
  .st-ok { color: rgb(var(--state-ok)); }
  .st-mod { color: rgb(var(--accent)); }
  .st-add { color: rgb(var(--state-ok)); }
  .st-del { color: rgb(var(--text-muted)); opacity: 0.5; }
  .st-conflict { color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }
</style>
