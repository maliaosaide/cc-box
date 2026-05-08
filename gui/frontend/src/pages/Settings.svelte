<script>
  import { onMount } from 'svelte'
  import { GetConfig, SetConfigField, TestConnection, BrowseFolder } from '../../wailsjs/go/main/App.js'

  export let syncState = 'idle'
  let activeTab = 'connection'

  const tabs = [
    { id: 'connection', label: '连接' },
    { id: 'encryption', label: '加密' },
    { id: 'sync', label: '同步' },
    { id: 'paths', label: '路径' },
    { id: 'exclude', label: '排除规则' },
    { id: 'devices', label: '设备' },
  ]

  let cfg = null
  let loading = true
  let error = ''
  let testResult = null
  let testLoading = false
  let saving = false
  let deviceName = ''

  // 默认排除规则
  const defaultPatterns = ['sessions/', 'cache/', 'debug/', 'telemetry/', 'downloads/', 'paste-cache/', 'shell-snapshots/', 'file-history/', 'session-env/', 'ide/', 'backups/', 'plans/', 'tasks/', 'teams/', 'plugins/data/', '*.lock']

  onMount(async () => {
    await loadConfig()
  })

  async function loadConfig() {
    loading = true; error = ''
    try {
      cfg = await GetConfig()
      if (cfg) deviceName = cfg.device.name
    } catch (e) {
      error = e.message || String(e)
    }
    loading = false
  }

  async function testConn() {
    testLoading = true; testResult = null
    try {
      testResult = await TestConnection()
    } catch (e) {
      testResult = { success: false, error: e.message || String(e) }
    }
    testLoading = false
  }

  async function saveField(section, key, value) {
    try {
      await SetConfigField(section, key, value)
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function saveDeviceName() {
    await saveField('device', 'name', deviceName)
  }

  function isDefaultPattern(p) {
    return defaultPatterns.includes(p)
  }
</script>

<div class="settings-page">
  <h1 class="section-title animate-fade-in">设置</h1>

  <div class="tabs-bar animate-fade-in">
    {#each tabs as tab}
      <button class="tab-btn" class:active={activeTab === tab.id} on:click={() => activeTab = tab.id}>
        {tab.label}
      </button>
    {/each}
  </div>

  {#if error}
    <div class="error-bar animate-fade-in">
      <span>{error}</span>
      <button class="link-btn" on:click={() => error = ''}>关闭</button>
    </div>
  {/if}

  {#if loading}
    <div class="flex items-center justify-center h-64">
      <div class="loading-dot animate-gentle-pulse"></div>
    </div>
  {:else if !cfg}
    <div class="card"><div class="empty-compact">无法加载配置</div></div>
  {:else}
    <!-- 连接 -->
    {#if activeTab === 'connection'}
      <div class="card animate-fade-in">
        <div class="form-group">
          <label class="label">WebDAV 服务地址</label>
          <input class="input" value={cfg.webdav.url} disabled />
        </div>
        <div class="form-group">
          <label class="label">用户名</label>
          <input class="input" value={cfg.webdav.username} disabled />
        </div>
        <div class="form-group">
          <label class="label">根路径</label>
          <input class="input" value={cfg.webdav.root} disabled />
        </div>
        <div class="form-actions">
          {#if testResult}
            <span class="test-result" class:ok={testResult.success} class:err={!testResult.success}>
              {testResult.success ? `已连接 (${testResult.latency}ms)` : `失败: ${testResult.error}`}
            </span>
          {/if}
          <button class="btn-primary" disabled={testLoading} on:click={testConn}>
            {testLoading ? '测试中...' : '测试连接'}
          </button>
        </div>
      </div>
    {/if}

    <!-- 加密 -->
    {#if activeTab === 'encryption'}
      <div class="card animate-fade-in">
        <div class="info-row">
          <span class="info-label">加密状态</span>
          <span class="badge-ok">{cfg.encryption.enabled ? '已启用' : '未启用'}</span>
        </div>
        <div class="info-row">
          <span class="info-label">算法</span>
          <span class="info-value font-mono">AES-256-GCM</span>
        </div>
        <div class="info-row">
          <span class="info-label">密钥派生</span>
          <span class="info-value font-mono">Argon2id</span>
        </div>
        <div class="warn-box">
          更改密码将重新加密所有云端数据，不可撤销
        </div>
      </div>
    {/if}

    <!-- 同步 -->
    {#if activeTab === 'sync'}
      <div class="card animate-fade-in">
        <div class="form-group">
          <label class="label">快照保留数</label>
          <input class="input" type="number" value={cfg.sync.snapshotLimit} disabled />
        </div>
        <div class="form-group">
          <label class="label">冲突策略</label>
          <input class="input" value={cfg.sync.conflictStrategy} disabled />
        </div>
        <div class="form-group">
          <label class="label">合并重试次数</label>
          <input class="input" type="number" value={cfg.sync.mergeRetryMax} disabled />
        </div>
      </div>
    {/if}

    <!-- 路径 -->
    {#if activeTab === 'paths'}
      <div class="card animate-fade-in">
        <div class="form-group">
          <label class="label">Claude 配置目录</label>
          <input class="input" value={cfg.claudeDir} disabled />
        </div>
        <div class="hint">留空使用默认路径 ~/.claude/</div>
      </div>
    {/if}

    <!-- 排除规则 -->
    {#if activeTab === 'exclude'}
      <div class="card animate-fade-in">
        <div class="exclude-header">
          <span class="info-label">排除的文件/目录</span>
          <span class="text-xs text-txt-muted">{cfg.exclude.length} 条规则</span>
        </div>
        <div class="exclude-list">
          {#each cfg.exclude as pattern}
            <div class="exclude-row">
              <span class="exclude-pattern font-mono">{pattern}</span>
              <span class="exclude-type" class:default={isDefaultPattern(pattern)} class:custom={!isDefaultPattern(pattern)}>
                {isDefaultPattern(pattern) ? '默认' : '自定义'}
              </span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- 设备 -->
    {#if activeTab === 'devices'}
      <div class="card animate-fade-in">
        <div class="device-card">
          <div class="device-main">
            <div class="device-badge">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="20" height="20">
                <rect x="2" y="3" width="20" height="14" rx="2"/>
                <path d="M8 21h8M12 17v4"/>
              </svg>
            </div>
            <div class="device-info">
              <span class="device-name">{cfg.device.name}</span>
              <span class="device-id font-mono">{cfg.device.id}</span>
            </div>
            <span class="device-current">本机</span>
          </div>
          <div class="device-actions">
            <input class="input" type="text" bind:value={deviceName} placeholder="设备名称" />
            <button class="btn-primary" on:click={saveDeviceName}>保存名称</button>
          </div>
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .settings-page { display: flex; flex-direction: column; gap: 12px; max-width: 640px; }

  .tabs-bar {
    display: flex; gap: 2px; padding: 4px;
    background: rgb(var(--surface-1)); border-radius: 8px; border: 1px solid rgb(var(--border));
  }
  .tab-btn {
    flex: 1; padding: 6px 8px; border-radius: 6px;
    font-size: 12px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--text-muted)); background: transparent;
    border: none; cursor: pointer; transition: all 0.2s;
  }
  .tab-btn:hover { color: rgb(var(--text-secondary)); }
  .tab-btn.active { background: rgba(196,112,78,0.1); color: rgb(var(--accent)); }

  .form-group { margin-bottom: 16px; }
  .form-group:last-child { margin-bottom: 0; }
  .hint { font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.6; margin-top: -8px; margin-bottom: 12px; }

  .form-actions {
    display: flex; align-items: center; justify-content: flex-end;
    gap: 12px; margin-top: 16px; padding-top: 12px;
    border-top: 1px solid rgb(var(--border));
  }

  .test-result { font-size: 11px; font-family: 'DM Mono', monospace; }
  .test-result.ok { color: rgb(var(--state-ok)); }
  .test-result.err { color: rgb(var(--state-err)); }

  .info-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: 10px 0; border-bottom: 1px solid rgba(46,45,51,0.4);
  }
  .info-row:last-child { border-bottom: none; }
  .info-label { font-size: 12px; color: rgb(var(--text-secondary)); }
  .info-value { font-size: 12px; color: rgb(var(--text-primary)); }

  .warn-box {
    margin-top: 12px; padding: 10px 14px; border-radius: 6px;
    font-size: 11px; color: rgb(var(--state-warn));
    background: rgba(196,165,78,0.06); border: 1px solid rgba(196,165,78,0.12);
  }

  .exclude-header {
    display: flex; align-items: center; justify-content: space-between;
    margin-bottom: 10px;
  }
  .exclude-list { display: flex; flex-direction: column; }
  .exclude-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: 6px 0; border-bottom: 1px solid rgba(46,45,51,0.3);
  }
  .exclude-row:last-child { border-bottom: none; }
  .exclude-pattern { font-size: 12px; color: rgb(var(--text-primary)); }
  .exclude-type {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 6px; border-radius: 3px;
  }
  .exclude-type.default { color: rgb(var(--text-muted)); background: rgb(var(--surface-2)); }
  .exclude-type.custom { color: rgb(var(--accent)); background: rgba(196,112,78,0.08); }

  .device-card { display: flex; flex-direction: column; gap: 12px; }
  .device-main { display: flex; align-items: center; gap: 12px; }
  .device-badge {
    width: 40px; height: 40px; border-radius: 8px;
    display: flex; align-items: center; justify-content: center;
    background: rgba(196,112,78,0.08); color: rgb(var(--accent));
  }
  .device-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
  .device-name { font-size: 14px; font-weight: 600; color: rgb(var(--text-primary)); }
  .device-id { font-size: 11px; color: rgb(var(--text-muted)); }
  .device-current {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 8px; border-radius: 3px;
    background: rgba(107,144,128,0.1); color: rgb(var(--state-ok));
  }
  .device-actions { display: flex; gap: 8px; align-items: center; }
  .device-actions .input { flex: 1; }

  .badge-ok {
    font-size: 11px; font-family: 'DM Mono', monospace;
    padding: 3px 10px; border-radius: 4px;
    background: rgba(107,144,128,0.1); color: rgb(var(--state-ok));
  }

  .error-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(184,92,92,0.08); border: 1px solid rgba(184,92,92,0.15);
    font-size: 12px; color: rgb(var(--state-err));
  }

  .btn-primary {
    font-size: 12px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: #0F0E11;
    background: linear-gradient(135deg, rgb(var(--accent)), rgba(196,112,78,0.8));
    border: none; padding: 6px 14px; border-radius: 6px; cursor: pointer;
    transition: opacity 0.2s;
  }
  .btn-primary:hover { opacity: 0.9; }
  .btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }

  .link-btn {
    font-size: 11px; color: rgb(var(--text-muted));
    background: none; border: none; cursor: pointer; transition: color 0.2s;
  }
  .link-btn:hover { color: rgb(var(--accent)); }

  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
</style>
