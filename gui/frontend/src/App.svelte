<script>
  import { onMount } from 'svelte'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import { IsInitialized, GetAppInfo } from '../wailsjs/go/main/App.js'
  import Sidebar from './lib/components/Sidebar.svelte'
  import Onboarding from './pages/Onboarding.svelte'
  import Dashboard from './pages/Dashboard.svelte'
  import Files from './pages/Files.svelte'
  import Binaries from './pages/Binaries.svelte'
  import Projects from './pages/Projects.svelte'
  import History from './pages/History.svelte'
  import Settings from './pages/Settings.svelte'

  let initialized = false
  let loading = true
  let currentPage = 'dashboard'
  let mountedPages = { dashboard: true }
  let syncState = 'idle'
  let theme = 'dark'
  let refreshVersions = { dashboard: 0, files: 0, binaries: 0, projects: 0, history: 0 }

  onMount(async () => {
    const saved = localStorage.getItem('cc-box-theme')
    if (saved) theme = saved
    document.documentElement.dataset.theme = theme
    try {
      initialized = await IsInitialized() || localStorage.getItem('cc-box-skip-onboarding') === 'true'
    } catch (e) { console.error('init check failed:', e) }
    EventsOn('data:changed', (e) => markChanged(e?.domain || 'all'))
    loading = false
  })

  function markChanged(domain) {
    const pages = pagesForDomain(domain)
    const next = { ...refreshVersions }
    pages.forEach(page => next[page] = (next[page] || 0) + 1)
    refreshVersions = next
  }

  function pagesForDomain(domain) {
    if (domain === 'files') return ['dashboard', 'files']
    if (domain === 'sync') return ['dashboard', 'files', 'history', 'binaries']
    if (domain === 'config') return ['dashboard', 'files', 'binaries']
    if (domain === 'binary') return ['dashboard', 'binaries']
    if (domain === 'projects') return ['projects']
    return ['dashboard', 'files', 'binaries', 'projects', 'history']
  }

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.dataset.theme = theme
    localStorage.setItem('cc-box-theme', theme)
  }

  $: if (!mountedPages[currentPage]) mountedPages = { ...mountedPages, [currentPage]: true }

  function handleInitComplete(e) {
    if (e.detail?.skipped) localStorage.setItem('cc-box-skip-onboarding', 'true')
    else localStorage.removeItem('cc-box-skip-onboarding')
    initialized = true
    currentPage = 'dashboard'
  }
  function navigate(e) { if (e.detail?.page) currentPage = e.detail.page }
</script>

{#if loading}
  <div class="loading-screen">
    <div class="loading-mark animate-gentle-pulse">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/>
        <path d="M2 17l10 5 10-5"/>
        <path d="M2 12l10 5 10-5"/>
      </svg>
    </div>
  </div>
{:else if !initialized}
  <div class="h-full">
    <Onboarding on:complete={handleInitComplete} />
  </div>
{:else}
  <div class="h-full flex flex-col">
    <div class="flex-1 flex overflow-hidden">
      <Sidebar bind:currentPage={currentPage} {syncState} {theme} on:navigate={() => {}} on:toggleTheme={toggleTheme} />
      <main class="main-content">
        {#if mountedPages.dashboard}
          <div class="page-panel" class:active={currentPage === 'dashboard'}>
            <Dashboard bind:syncState {theme} active={currentPage === 'dashboard'} refreshToken={refreshVersions.dashboard} on:navigate={navigate} on:toggleTheme={toggleTheme} />
          </div>
        {/if}
        {#if mountedPages.files}
          <div class="page-panel" class:active={currentPage === 'files'}>
            <Files bind:syncState active={currentPage === 'files'} refreshToken={refreshVersions.files} on:navigate={navigate} />
          </div>
        {/if}
        {#if mountedPages.binaries}
          <div class="page-panel" class:active={currentPage === 'binaries'}>
            <Binaries active={currentPage === 'binaries'} refreshToken={refreshVersions.binaries} on:navigate={navigate} />
          </div>
        {/if}
        {#if mountedPages.projects}
          <div class="page-panel" class:active={currentPage === 'projects'}>
            <Projects active={currentPage === 'projects'} refreshToken={refreshVersions.projects} on:navigate={navigate} />
          </div>
        {/if}
        {#if mountedPages.history}
          <div class="page-panel" class:active={currentPage === 'history'}>
            <History active={currentPage === 'history'} refreshToken={refreshVersions.history} on:navigate={navigate} />
          </div>
        {/if}
        {#if mountedPages.settings}
          <div class="page-panel" class:active={currentPage === 'settings'}>
            <Settings on:navigate={navigate} />
          </div>
        {/if}
      </main>
    </div>
  </div>
{/if}

<style>
  .loading-screen {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgb(var(--surface-0));
  }

  .loading-mark {
    width: 48px;
    height: 48px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(196,112,78,0.08);
    color: rgb(var(--accent));
  }
  .loading-mark svg { width: 24px; height: 24px; }

  .main-content {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .page-panel {
    display: none;
    flex: 1;
    overflow: hidden;
    padding: 28px 32px;
    overflow-y: auto;
  }
  .page-panel.active {
    display: flex;
    flex-direction: column;
  }
</style>
