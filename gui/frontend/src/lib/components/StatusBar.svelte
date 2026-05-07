<script>
  export let syncState = 'idle'
</script>

<footer class="statusbar">
  <div class="flex items-center gap-3">
    <div class="flex items-center gap-2">
      <div class="sb-dot"
           class:ok={syncState === 'idle' || syncState === 'synced'}
           class:syncing={syncState === 'syncing'}
           class:err={syncState === 'error'}
           class:warn={syncState === 'conflict'}>
      </div>
      <span class="sb-text">
        {#if syncState === 'idle' || syncState === 'synced'}已同步
        {:else if syncState === 'syncing'}同步中...
        {:else if syncState === 'conflict'}存在冲突
        {:else if syncState === 'error'}连接异常
        {:else}未连接
        {/if}
      </span>
    </div>
  </div>
  <span class="sb-version">v0.1.0</span>
</footer>

<style>
  .statusbar {
    height: 28px;
    background: rgb(var(--surface-1));
    border-top: 1px solid rgb(var(--border));
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
  }

  .sb-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: rgb(var(--state-ok));
    transition: background 0.3s;
  }
  .sb-dot.syncing {
    background: rgb(var(--state-sync));
    animation: gentle-pulse 1.5s ease-in-out infinite;
  }
  .sb-dot.err { background: rgb(var(--state-err)); }
  .sb-dot.warn { background: rgb(var(--state-warn)); }

  .sb-text {
    font-size: 10px;
    font-family: 'DM Mono', monospace;
    color: rgb(var(--text-muted));
  }

  .sb-version {
    font-size: 10px;
    font-family: 'DM Mono', monospace;
    color: rgb(var(--text-muted));
    opacity: 0.5;
  }

  @keyframes gentle-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }
</style>
