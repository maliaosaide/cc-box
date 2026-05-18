<script>
  import { onMount } from 'svelte'
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
  let syncState = 'idle'
  let theme = 'dark'

  onMount(async () => {
    const saved = localStorage.getItem('cc-box-theme')
    if (saved) theme = saved
    document.documentElement.dataset.theme = theme
    try {
      initialized = await IsInitialized()
    } catch (e) { console.error('init check failed:', e) }
    loading = false
  })

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.dataset.theme = theme
    localStorage.setItem('cc-box-theme', theme)
  }

  function handleInitComplete() { initialized = true; currentPage = 'dashboard' }
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
        <div class="page-panel" class:active={currentPage === 'dashboard'}>
          <Dashboard bind:syncState {theme} on:navigate={navigate} on:toggleTheme={toggleTheme} />
        </div>
        <div class="page-panel" class:active={currentPage === 'files'}>
          <Files bind:syncState on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'binaries'}>
          <Binaries on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'projects'}>
          <Projects on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'history'}>
          <History on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'settings'}>
          <Settings on:navigate={navigate} />
        </div>
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
