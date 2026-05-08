<script>
  import { onMount } from 'svelte'
  import { GetConfig, SetConfigField, TestConnection, SetWebDAVPassword, AddExcludePattern, RemoveExcludePattern } from '../../wailsjs/go/main/App.js'

  export let syncState = 'idle'
  let activeTab = 'connection'

  const tabs = [
    { id: 'connection', label: '连接' },
    { id: 'encryption', label: '加密' },
    { id: 'binary', label: '二进制' },
    { id: 'sync', label: '同步' },
    { id: 'paths', label: '路径' },
    { id: 'exclude', label: '排除规则' },
    { id: 'devices', label: '设备' },
  ]

  let cfg = null
  let loading = true
  let error = ''
  let saved = ''
  let testResult = null
  let testLoading = false

  // 可编辑字段
  let webdavUrl = ''
  let webdavUser = ''
  let webdavPassNew = ''
  let deviceName = ''
  let encryptionEnabled = false
  let binaryEncrypt = false
  let chunkMode = 'auto'
  let chunkSizeMB = 10
  let chunkThresholdMB = 50
  let autoUpload = false
  let snapshotLimit = 50
  let conflictStrategy = 'ask'
  let mergeRetryMax = 3
  let claudeDirRaw = ''
  let binDirRaw = ''
  let versionsDirRaw = ''
  let newPattern = ''
  let excludeList = []

  const defaultPatterns = ['sessions/', 'cache/', 'debug/', 'telemetry/', 'downloads/', 'paste-cache/', 'shell-snapshots/', 'file-history/', 'session-env/', 'ide/', 'backups/', 'plans/', 'tasks/', 'teams/', 'plugins/data/', '*.lock']

  onMount(async () => {
    await loadConfig()
  })

  async function loadConfig() {
    loading = true; error = ''
    try {
      cfg = await GetConfig()
      if (cfg) {
        webdavUrl = cfg.webdav.url || ''
        webdavUser = cfg.webdav.username || ''
        deviceName = cfg.device.name || ''
        encryptionEnabled = cfg.encryption.enabled
        binaryEncrypt = cfg.binary.encrypt
        chunkMode = cfg.binary.chunkMode || 'auto'
        chunkSizeMB = cfg.binary.chunkSizeMB || 10
        chunkThresholdMB = cfg.binary.chunkThresholdMB || 50
        autoUpload = cfg.binary.autoUpload
        snapshotLimit = cfg.sync.snapshotLimit || 50
        conflictStrategy = cfg.sync.conflictStrategy || 'ask'
        mergeRetryMax = cfg.sync.mergeRetryMax || 3
        claudeDirRaw = cfg.claudeDirRaw || ''
        binDirRaw = cfg.binDirRaw || ''
        versionsDirRaw = cfg.versionsDirRaw || ''
        excludeList = [...(cfg.exclude || [])]
      }
    } catch (e) {
      error = e.message || String(e)
    }
    loading = false
  }

  async function saveField(section, key, value) {
    try {
      await SetConfigField(section, key, value)
      showSaved()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function saveWebdavUrl() { await saveField('webdav', 'url', webdavUrl) }
  async function saveWebdavUser() { await saveField('webdav', 'username', webdavUser) }
  async function saveDeviceName() { await saveField('device', 'name', deviceName) }

  async function savePassword() {
    if (!webdavPassNew) return
    try {
      await SetWebDAVPassword(webdavPassNew)
      webdavPassNew = ''
      if (cfg) cfg.webdav.hasPassword = true
      showSaved()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function toggleEncryption() {
    encryptionEnabled = !encryptionEnabled
    await saveField('encryption', 'enabled', String(encryptionEnabled))
  }

  async function toggleBinaryEncrypt() {
    binaryEncrypt = !binaryEncrypt
    await saveField('binary', 'encrypt', String(binaryEncrypt))
  }

  async function toggleAutoUpload() {
    autoUpload = !autoUpload
    await saveField('binary', 'auto_upload', String(autoUpload))
  }

  async function saveChunkMode() { await saveField('binary', 'chunk_mode', chunkMode) }
  async function saveChunkSize() { await saveField('binary', 'chunk_size_mb', String(chunkSizeMB)) }
  async function saveChunkThreshold() { await saveField('binary', 'chunk_threshold_mb', String(chunkThresholdMB)) }
  async function saveSnapshotLimit() { await saveField('sync', 'snapshot_limit', String(snapshotLimit)) }
  async function saveConflictStrategy() { await saveField('sync', 'conflict_strategy', conflictStrategy) }
  async function saveMergeRetry() { await saveField('sync', 'merge_retry_max', String(mergeRetryMax)) }
  async function saveClaudeDir() { await saveField('claude', 'path', claudeDirRaw) }
  async function saveBinDir() { await saveField('binary', 'bin_dir', binDirRaw) }
  async function saveVersionsDir() { await saveField('binary', 'versions_dir', versionsDirRaw) }

  async function addPattern() {
    const p = newPattern.trim()
    if (!p) return
    try {
      await AddExcludePattern(p)
      excludeList = [...excludeList, p]
      newPattern = ''
      showSaved()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function removePattern(p) {
    try {
      await RemoveExcludePattern(p)
      excludeList = excludeList.filter(x => x !== p)
      showSaved()
    } catch (e) {
      error = e.message || String(e)
    }
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

  function showSaved() {
    saved = '已保存'
    setTimeout(() => saved = '', 1500)
  }

  function isDefaultPattern(p) {
    return defaultPatterns.includes(p)
  }
</script>

<div class="settings-page">
  <div class="toolbar animate-fade-in">
    <h1 class="section-title">设置</h1>
    {#if saved}
      <span class="saved-hint">{saved}</span>
    {/if}
  </div>

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
          <div class="input-row">
            <input class="input" type="text" bind:value={webdavUrl} placeholder="https://your-server/dav/" />
            <button class="btn-sm" on:click={saveWebdavUrl}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label">用户名</label>
          <div class="input-row">
            <input class="input" type="text" bind:value={webdavUser} placeholder="用户名" />
            <button class="btn-sm" on:click={saveWebdavUser}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label">密码</label>
          <div class="input-row">
            <input class="input" type="password" bind:value={webdavPassNew} placeholder={cfg.webdav.hasPassword ? '••••••••（已保存）' : '输入密码'} />
            <button class="btn-sm" on:click={savePassword}>保存</button>
          </div>
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
        <div class="toggle-row">
          <div class="toggle-info">
            <span class="info-label">加密同步</span>
            <span class="info-desc">使用 AES-256-GCM 加密云端数据</span>
          </div>
          <button class="toggle-btn" class:on={encryptionEnabled} on:click={toggleEncryption}>
            <span class="toggle-knob"></span>
          </button>
        </div>
        <div class="info-row">
          <span class="info-label">加密算法</span>
          <span class="info-value font-mono">AES-256-GCM</span>
        </div>
        <div class="info-row">
          <span class="info-label">密钥派生</span>
          <span class="info-value font-mono">Argon2id</span>
        </div>
        <div class="warn-box">
          禁用加密后新上传的数据将不加密。已有的加密数据在下次同步时会被重新写入。
        </div>
      </div>
    {/if}

    <!-- 二进制 -->
    {#if activeTab === 'binary'}
      <div class="card animate-fade-in">
        <div class="toggle-row">
          <div class="toggle-info">
            <span class="info-label">二进制加密</span>
            <span class="info-desc">上传二进制文件时进行加密</span>
          </div>
          <button class="toggle-btn" class:on={binaryEncrypt} on:click={toggleBinaryEncrypt}>
            <span class="toggle-knob"></span>
          </button>
        </div>
        <div class="toggle-row">
          <div class="toggle-info">
            <span class="info-label">自动上传</span>
            <span class="info-desc">同步时自动上传变更的二进制文件</span>
          </div>
          <button class="toggle-btn" class:on={autoUpload} on:click={toggleAutoUpload}>
            <span class="toggle-knob"></span>
          </button>
        </div>
        <div class="form-group">
          <label class="label">分块模式</label>
          <select class="input select-input" bind:value={chunkMode} on:change={saveChunkMode}>
            <option value="auto">自动</option>
            <option value="always">始终分块</option>
            <option value="never">不分块</option>
          </select>
        </div>
        <div class="form-group">
          <label class="label">分块大小 (MB)</label>
          <div class="input-row">
            <input class="input" type="number" bind:value={chunkSizeMB} min="1" max="100" />
            <button class="btn-sm" on:click={saveChunkSize}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label">分块阈值 (MB)</label>
          <div class="input-row">
            <input class="input" type="number" bind:value={chunkThresholdMB} min="1" max="500" />
            <button class="btn-sm" on:click={saveChunkThreshold}>保存</button>
          </div>
          <div class="hint">超过此大小的文件在自动模式下将分块上传</div>
        </div>
      </div>
    {/if}

    <!-- 同步 -->
    {#if activeTab === 'sync'}
      <div class="card animate-fade-in">
        <div class="form-group">
          <label class="label">快照保留数</label>
          <div class="input-row">
            <input class="input" type="number" bind:value={snapshotLimit} min="5" max="200" />
            <button class="btn-sm" on:click={saveSnapshotLimit}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label">冲突策略</label>
          <select class="input select-input" bind:value={conflictStrategy} on:change={saveConflictStrategy}>
            <option value="ask">询问</option>
            <option value="local">保留本地</option>
            <option value="remote">采用远程</option>
            <option value="merge">尝试合并</option>
          </select>
        </div>
        <div class="form-group">
          <label class="label">合并重试次数</label>
          <div class="input-row">
            <input class="input" type="number" bind:value={mergeRetryMax} min="1" max="10" />
            <button class="btn-sm" on:click={saveMergeRetry}>保存</button>
          </div>
        </div>
      </div>
    {/if}

    <!-- 路径 -->
    {#if activeTab === 'paths'}
      <div class="card animate-fade-in">
        <div class="form-group">
          <label class="label">Claude 配置目录</label>
          <div class="input-row">
            <input class="input font-mono" type="text" bind:value={claudeDirRaw} placeholder="留空使用默认 ~/.claude/" />
            <button class="btn-sm" on:click={saveClaudeDir}>保存</button>
          </div>
          <div class="hint">当前解析路径: {cfg.claudeDir}</div>
        </div>
        <div class="form-group">
          <label class="label">Claude 二进制目录</label>
          <div class="input-row">
            <input class="input font-mono" type="text" bind:value={binDirRaw} placeholder="留空使用默认 ~/.local/bin/" />
            <button class="btn-sm" on:click={saveBinDir}>保存</button>
          </div>
          <div class="hint">当前解析路径: {cfg.binDir}</div>
        </div>
        <div class="form-group">
          <label class="label">Claude 版本目录</label>
          <div class="input-row">
            <input class="input font-mono" type="text" bind:value={versionsDirRaw} placeholder="留空使用默认 ~/.local/share/claude/versions/" />
            <button class="btn-sm" on:click={saveVersionsDir}>保存</button>
          </div>
          <div class="hint">当前解析路径: {cfg.versionsDir}</div>
        </div>
      </div>
    {/if}

    <!-- 排除规则 -->
    {#if activeTab === 'exclude'}
      <div class="card animate-fade-in">
        <div class="exclude-header">
          <span class="info-label">排除的文件/目录</span>
          <span class="text-xs text-txt-muted">{excludeList.length} 条规则</span>
        </div>
        <div class="add-row">
          <input class="input" type="text" bind:value={newPattern} placeholder="输入规则，如 node_modules/ 或 *.log" on:keydown={(e) => { if (e.key === 'Enter') addPattern() }} />
          <button class="btn-sm" on:click={addPattern}>添加</button>
        </div>
        <div class="exclude-list">
          {#each excludeList as pattern}
            <div class="exclude-row">
              <div class="exclude-left">
                <span class="exclude-pattern font-mono">{pattern}</span>
                <span class="exclude-type" class:default={isDefaultPattern(pattern)} class:custom={!isDefaultPattern(pattern)}>
                  {isDefaultPattern(pattern) ? '默认' : '自定义'}
                </span>
              </div>
              <button class="del-btn" on:click={() => removePattern(pattern)}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
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
  .toolbar { display: flex; align-items: center; justify-content: space-between; }
  .saved-hint { font-size: 11px; color: rgb(var(--state-ok)); font-family: 'DM Mono', monospace; }

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
  .label { display: block; font-size: 12px; color: rgb(var(--text-secondary)); margin-bottom: 6px; }
  .hint { font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.6; margin-top: 4px; }

  .input-row { display: flex; gap: 8px; align-items: center; }
  .input-row .input { flex: 1; }

  .select-input { appearance: none; padding-right: 28px; background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%237a7880' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E"); background-repeat: no-repeat; background-position: right 8px center; }

  .form-actions {
    display: flex; align-items: center; justify-content: flex-end;
    gap: 12px; margin-top: 16px; padding-top: 12px;
    border-top: 1px solid rgb(var(--border));
  }

  .test-result { font-size: 11px; font-family: 'DM Mono', monospace; }
  .test-result.ok { color: rgb(var(--state-ok)); }
  .test-result.err { color: rgb(var(--state-err)); }

  .toggle-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: 12px 0; border-bottom: 1px solid rgba(46,45,51,0.4);
  }
  .toggle-row:last-of-type { border-bottom: none; }
  .toggle-info { display: flex; flex-direction: column; gap: 2px; }
  .toggle-desc { font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.6; }

  .toggle-btn {
    width: 36px; height: 20px; border-radius: 10px;
    background: rgb(var(--surface-2)); border: 1px solid rgb(var(--border));
    position: relative; cursor: pointer; transition: all 0.2s;
    flex-shrink: 0;
  }
  .toggle-btn.on { background: rgba(196,112,78,0.3); border-color: rgb(var(--accent)); }
  .toggle-knob {
    position: absolute; top: 2px; left: 2px;
    width: 14px; height: 14px; border-radius: 50%;
    background: rgb(var(--text-muted)); transition: all 0.2s;
  }
  .toggle-btn.on .toggle-knob { left: 18px; background: rgb(var(--accent)); }

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
  .add-row {
    display: flex; gap: 8px; margin-bottom: 12px;
  }
  .add-row .input { flex: 1; }
  .exclude-list { display: flex; flex-direction: column; }
  .exclude-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: 6px 0; border-bottom: 1px solid rgba(46,45,51,0.3);
  }
  .exclude-row:last-child { border-bottom: none; }
  .exclude-left { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
  .exclude-pattern { font-size: 12px; color: rgb(var(--text-primary)); }
  .exclude-type {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 6px; border-radius: 3px; flex-shrink: 0;
  }
  .exclude-type.default { color: rgb(var(--text-muted)); background: rgb(var(--surface-2)); }
  .exclude-type.custom { color: rgb(var(--accent)); background: rgba(196,112,78,0.08); }
  .del-btn {
    width: 24px; height: 24px; border-radius: 4px;
    display: flex; align-items: center; justify-content: center;
    background: transparent; border: none; cursor: pointer;
    color: rgb(var(--text-muted)); opacity: 0.4; transition: all 0.2s;
    flex-shrink: 0;
  }
  .del-btn:hover { opacity: 1; color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }

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

  .btn-sm {
    font-size: 11px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--accent));
    background: rgba(196,112,78,0.08);
    border: 1px solid rgba(196,112,78,0.2);
    padding: 5px 12px; border-radius: 5px; cursor: pointer;
    transition: all 0.2s; white-space: nowrap;
  }
  .btn-sm:hover { background: rgba(196,112,78,0.15); }

  .link-btn {
    font-size: 11px; color: rgb(var(--text-muted));
    background: none; border: none; cursor: pointer; transition: color 0.2s;
  }
  .link-btn:hover { color: rgb(var(--accent)); }

  .error-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(184,92,92,0.08); border: 1px solid rgba(184,92,92,0.15);
    font-size: 12px; color: rgb(var(--state-err));
  }

  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
</style>
