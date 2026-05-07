<script>
  import { onMount } from 'svelte'
  import { IsInitialized, GetAppInfo } from '../wailsjs/go/main/App.js'
  import Sidebar from './lib/components/Sidebar.svelte'
  import StatusBar from './lib/components/StatusBar.svelte'
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

  onMount(async () => {
    try {
      initialized = await IsInitialized()
    } catch (e) { console.error('init check failed:', e) }
    loading = false
  })

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
      <Sidebar bind:currentPage={currentPage} on:navigate={() => {}} />
      <main class="main-content">
        <div class="page-panel" class:active={currentPage === 'dashboard'}>
          <Dashboard bind:syncState on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'files'}>
          <Files bind:syncState on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'binaries'}>
          <Binaries bind:syncState on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'projects'}>
          <Projects bind:syncState on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'history'}>
          <History bind:syncState on:navigate={navigate} />
        </div>
        <div class="page-panel" class:active={currentPage === 'settings'}>
          <Settings bind:syncState on:navigate={navigate} />
        </div>
      </main>
    </div>
    <StatusBar {syncState} />
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
    overflow-y: auto;
    padding: 28px 32px;
    position: relative;
  }

  .page-panel {
    display: none;
    height: 100%;
  }
  .page-panel.active {
    display: block;
  }
</style>
