<script>
  import { createEventDispatcher } from 'svelte'

  export let currentPage = 'dashboard'
  export let syncState = 'idle'
  export let theme = 'dark'
  const dispatch = createEventDispatcher()

  const navItems = [
    { id: 'dashboard', label: '概览', icon: 'grid' },
    { id: 'files', label: '配置', icon: 'folder' },
    { id: 'binaries', label: '二进制', icon: 'package' },
    { id: 'projects', label: '项目', icon: 'layers' },
    { id: 'history', label: '历史', icon: 'clock' },
    { id: 'settings', label: '设置', icon: 'settings' },
  ]

  $: isWarnState = syncState === 'conflict' || syncState === 'pending' || syncState === 'remote_uninitialized' || syncState === 'idle'
  $: isErrorState = syncState === 'error' || syncState === 'connection_error' || syncState === 'remote_incomplete' || syncState === 'key_mismatch' || syncState === 'local_error'

  function select(id) {
    currentPage = id
    dispatch('navigate', { page: id })
  }
</script>

<nav class="sidebar">
  <!-- Logo -->
  <div class="sidebar-logo">
    <div class="logo-mark">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/>
        <path d="M2 17l10 5 10-5"/>
        <path d="M2 12l10 5 10-5"/>
      </svg>
    </div>
    <div class="logo-text">
      <span class="font-display font-bold text-base tracking-tight text-txt-primary">CC-Box</span>
    </div>
  </div>

  <!-- Navigation -->
  <div class="sidebar-nav">
    {#each navItems as item}
      <button class="nav-item" class:active={currentPage === item.id}
              on:click={() => select(item.id)}>
        {#if currentPage === item.id}
          <div class="nav-ribbon"></div>
        {/if}
        <svg class="nav-icon" viewBox="0 0 20 20" fill="currentColor">
          {#if item.icon === 'grid'}
            <path d="M3 3h5v5H3V3zm9 0h5v5h-5V3zm-9 9h5v5H3v-5zm9 0h5v5h-5v-5z"/>
          {:else if item.icon === 'folder'}
            <path d="M2 4.75A.75.75 0 012.75 4h4.19l.94 1.06h7.36a.75.75 0 010 1.5H2.75A.75.75 0 012 5.81V4.75zm0 3.5a.75.75 0 01.75-.75h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 8.25v-.5zm0 3.5a.75.75 0 01.75-.75h14.5a.75.75 0 010 1.5H2.75a.75.75 0 01-.75-.75v-.5z"/>
          {:else if item.icon === 'package'}
            <path d="M10 1.5l-8 4v9l8 4 8-4v-9l-8-4zm0 2.3l5.5 2.75-5.5 2.75L4.5 6.55 10 3.8zM3 7.8l6 3v6.4l-6-3V7.8zm8 9.4v-6.4l6-3v6.4l-6 3z"/>
          {:else if item.icon === 'layers'}
            <path d="M10 1.5l8.5 4.25-8.5 4.25L1.5 5.75 10 1.5zM3.5 7.25L10 10.5l6.5-3.25v4.5L10 15l-6.5-3.25v-4.5zm-2 4L10 14.5l8.5-3.25v5L10 19.5 1.5 16.25v-5z"/>
          {:else if item.icon === 'clock'}
            <path d="M10 1.5a8.5 8.5 0 100 17 8.5 8.5 0 000-17zm0 1.5a7 7 0 110 14 7 7 0 010-14zm-.75 2.5v5.25l3.5 2.1.75-1.24-2.75-1.65V5.5H9.25z"/>
          {:else if item.icon === 'settings'}
            <path d="M8.17 1.5l-.42 2.37a6.5 6.5 0 00-1.72.99L3.7 3.92 2.37 6.22l1.79 1.6A6.5 6.5 0 004 9.5c0 .6.08 1.18.22 1.73l-1.79 1.6 1.33 2.3 2.33-.94c.51.4 1.08.73 1.69.97l.42 2.34h3.66l.42-2.37a6.5 6.5 0 001.72-.99l2.33.94 1.33-2.3-1.79-1.6c.14-.55.22-1.13.22-1.73s-.08-1.18-.22-1.73l1.79-1.6-1.33-2.3-2.33.94a6.5 6.5 0 00-1.69-.97L11.83 1.5H8.17zM10 7.5a2 2 0 110 4 2 2 0 010-4z"/>
          {/if}
        </svg>
        <span>{item.label}</span>
      </button>
    {/each}
  </div>

  <!-- Sync status -->
  <div class="sidebar-status" class:syncing={syncState === 'syncing'} class:warn={isWarnState} class:err={isErrorState}>
    <div class="status-dot" class:syncing={syncState === 'syncing'} class:warn={isWarnState} class:err={isErrorState}></div>
    <span class="status-text">
      {#if syncState === 'syncing'}同步中...
      {:else if syncState === 'conflict'}存在冲突
      {:else if syncState === 'pending'}待同步
      {:else if syncState === 'remote_uninitialized'}远程未初始化
      {:else if syncState === 'remote_incomplete'}远程数据不完整
      {:else if syncState === 'key_mismatch'}密钥不匹配
      {:else if syncState === 'connection_error' || syncState === 'error'}连接异常
      {:else if syncState === 'local_error'}本地配置异常
      {:else if syncState === 'synced'}已同步
      {:else}未同步{/if}
    </span>
    <button class="theme-toggle" on:click={() => dispatch('toggleTheme')} title={theme === 'dark' ? '切换亮色' : '切换暗色'}>
      {#if theme === 'dark'}
        <svg viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z"/></svg>
      {:else}
        <svg viewBox="0 0 20 20" fill="currentColor"><path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z"/></svg>
      {/if}
    </button>
  </div>
</nav>

<style>
  .sidebar {
    width: 220px;
    height: 100%;
    background: rgb(var(--surface-1));
    border-right: 1px solid rgb(var(--border));
    display: flex;
    flex-direction: column;
    position: relative;
  }

  .sidebar-logo {
    padding: 20px 18px 18px;
    display: flex;
    align-items: center;
    gap: 10px;
    border-bottom: 1px solid rgb(var(--border));
  }

  .logo-mark {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(196,112,78,0.1);
    color: rgb(var(--accent));
  }
  .logo-mark svg { width: 18px; height: 18px; }

  .sidebar-nav {
    flex: 1;
    padding: 10px 8px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .nav-item {
    position: relative;
    width: 100%;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 14px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    color: rgb(var(--text-secondary));
    background: transparent;
    border: none;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.22, 1, 0.36, 1);
  }
  .nav-item:hover {
    background: rgb(var(--surface-2));
    color: rgb(var(--text-primary));
  }
  .nav-item.active {
    background: rgba(196,112,78,0.06);
    color: rgb(var(--accent));
    font-weight: 600;
  }

  .nav-ribbon {
    position: absolute;
    left: 0;
    top: 50%;
    transform: translateY(-50%);
    width: 3px;
    height: 20px;
    border-radius: 0 3px 3px 0;
    background: rgb(var(--accent));
  }

  .nav-icon {
    width: 16px;
    height: 16px;
    flex-shrink: 0;
  }

  .sidebar-status {
    padding: 14px 18px;
    border-top: 1px solid rgb(var(--border));
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: rgb(var(--state-ok));
  }
  .status-dot.syncing { background: rgb(var(--state-sync)); }
  .status-dot.warn { background: rgb(var(--state-warn)); }
  .status-dot.err { background: rgb(var(--state-err)); }

  .sidebar-status.syncing .status-text { color: rgb(var(--state-sync)); }
  .sidebar-status.warn .status-text { color: rgb(var(--state-warn)); }
  .sidebar-status.err .status-text { color: rgb(var(--state-err)); }

  .status-text {
    font-size: 11px;
    font-family: 'DM Mono', monospace;
    color: rgb(var(--text-muted));
  }

  .theme-toggle {
    margin-left: auto;
    width: 24px; height: 24px;
    border-radius: 5px; border: none;
    display: flex; align-items: center; justify-content: center;
    background: transparent; cursor: pointer;
    color: rgb(var(--text-muted)); opacity: 0.5;
    transition: all 0.2s;
  }
  .theme-toggle:hover { opacity: 1; color: rgb(var(--accent)); background: rgb(var(--surface-2)); }
  .theme-toggle svg { width: 14px; height: 14px; }
</style>
