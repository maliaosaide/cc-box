<script>
  import { onMount } from 'svelte'
  import { GetConfig, SetConfigField, TestConnection, SetWebDAVPassword, AddExcludePattern, RemoveExcludePattern, GetClaudeDirectories, GetClaudeExcludeFiles, GetEncryptionStatus, VerifyEncryptionKey, ChangeEncryptionPassword, PreviewEncryptionPassword, SaveEncryptionPassword } from '../../wailsjs/go/main/App.js'

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
  let webdavRoot = ''
  let webdavProxyUrl = ''
  let webdavPassNew = ''
  let deviceName = ''
  let encryptionEnabled = false
  let binaryEncrypt = false
  let chunkMode = 'auto'
  let chunkSizeMB = 10
  let chunkThresholdMB = 50
  let binarySyncEnabled = false
  let binaryVerifySignature = false
  let autoConfigurePath = false
  let snapshotLimit = 50
  let conflictStrategy = 'ask'
  let mergeRetryMax = 3
  let autoSyncInterval = ''
  let pathInputs = {
    claudeDir: '',
    claudeBinary: '',
    claudeJSON: '',
  }
  let excludeList = []
  let claudeDirs = []
  let claudeFiles = []
  let encStatus = null
  let excludeLoaded = false
  let encryptionLoaded = false
  let verifyResult = null
  let verifyLoading = false
  let inputEncPass = ''
  let inputEncPreview = null
  let inputEncPreviewLoading = false
  let inputEncPreviewTimer = null
  let inputEncPreviewRequest = 0
  let saveEncPassLoading = false
  let oldEncPass = ''
  let oldEncPreview = null
  let oldEncPreviewLoading = false
  let oldEncPreviewTimer = null
  let oldEncPreviewRequest = 0
  let newEncPass = ''
  let confirmEncPass = ''
  let changePassLoading = false

  const defaultPatterns = []

  $: availableClaudeDirs = claudeDirs.filter(dir => !dir.excluded)
  $: availableClaudeFiles = claudeFiles.filter(file => !file.excluded)
  $: excludedDirectoryPatterns = excludeList.filter(isDirectoryPattern)
  $: excludedFilePatterns = excludeList.filter(isManagedFilePattern)
  $: if (cfg && activeTab === 'exclude') ensureExcludeLoaded()
  $: if (cfg && activeTab === 'encryption') ensureEncryptionStatus()

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
        webdavRoot = cfg.webdav.root || ''
        webdavProxyUrl = cfg.webdav.proxyUrl || ''
        deviceName = cfg.device.name || ''
        encryptionEnabled = cfg.encryption.enabled
        binaryEncrypt = cfg.binary.encrypt
        chunkMode = cfg.binary.chunkMode || 'auto'
        chunkSizeMB = cfg.binary.chunkSizeMB || 10
        chunkThresholdMB = cfg.binary.chunkThresholdMB || 50
        binarySyncEnabled = cfg.binary.syncEnabled ?? cfg.binary.autoUpload
        autoConfigurePath = cfg.binary.autoConfigurePath || false
        binaryVerifySignature = cfg.binary.verifySignature || false
        snapshotLimit = cfg.sync.snapshotLimit || 50
        conflictStrategy = cfg.sync.conflictStrategy || 'ask'
        mergeRetryMax = cfg.sync.mergeRetryMax || 3
        autoSyncInterval = cfg.sync.autoSyncInterval || ''
        pathInputs = {
          claudeDir: cfg.claudeDirRaw || '',
          claudeBinary: cfg.claudeBinaryPathRaw || '',
          claudeJSON: cfg.claudeJSONPathRaw || '',
        }
        excludeList = [...(cfg.exclude || [])]
        excludeLoaded = false
        encryptionLoaded = false
      }
    } catch (e) {
      error = e.message || String(e)
    }
    loading = false
  }

  $: webdavPathDirty = cfg && (webdavUrl.trim() !== (cfg.webdav.url || '') || normalizeWebDAVRoot(webdavRoot) !== (cfg.webdav.root || ''))

  function normalizeWebDAVRoot(root) {
    return String(root || '')
      .trim()
      .replace(/\\/g, '/')
      .replace(/^\/+|\/+$/g, '')
      .split('/')
      .map(part => part.trim())
      .filter(Boolean)
      .join('/')
  }

  async function saveField(section, key, value) {
    try {
      await SetConfigField(section, key, value)
      showSaved()
      return true
    } catch (e) {
      error = e.message || String(e)
      return false
    }
  }

  async function saveWebdavUrl() { if (await saveField('webdav', 'url', webdavUrl)) await loadConfig() }
  async function saveWebdavUser() { await saveField('webdav', 'username', webdavUser) }
  async function saveWebdavRoot() { if (await saveField('webdav', 'root', webdavRoot)) await loadConfig() }
  async function saveWebdavProxyUrl() { if (await saveField('webdav', 'proxy_url', webdavProxyUrl)) await loadConfig() }
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
    error = '加密和明文同步互斥，初始化后不能直接切换；如需切换，请新建同步组或执行完整迁移。'
    encryptionEnabled = cfg.encryption.enabled
  }

  async function toggleBinaryEncrypt() {
    binaryEncrypt = !binaryEncrypt
    await saveField('binary', 'encrypt', String(binaryEncrypt))
  }

  async function toggleBinarySync() {
    binarySyncEnabled = !binarySyncEnabled
    await saveField('binary', 'sync_enabled', String(binarySyncEnabled))
  }

  async function toggleAutoConfigurePath() {
    autoConfigurePath = !autoConfigurePath
    await saveField('binary', 'auto_configure_path', String(autoConfigurePath))
  }

  async function toggleVerifySignature() {
    binaryVerifySignature = !binaryVerifySignature
    await saveField('binary', 'verify_signature', String(binaryVerifySignature))
  }
  async function saveChunkMode() { await saveField('binary', 'chunk_mode', chunkMode) }
  async function saveChunkSize() { await saveField('binary', 'chunk_size_mb', String(chunkSizeMB)) }
  async function saveChunkThreshold() { await saveField('binary', 'chunk_threshold_mb', String(chunkThresholdMB)) }
  async function saveSnapshotLimit() { await saveField('sync', 'snapshot_limit', String(snapshotLimit)) }
  async function saveConflictStrategy() { await saveField('sync', 'conflict_strategy', conflictStrategy) }
  async function saveMergeRetry() { await saveField('sync', 'merge_retry_max', String(mergeRetryMax)) }
  async function saveAutoSyncInterval() { await saveField('sync', 'auto_sync_interval', autoSyncInterval) }

  function setPathInput(key, value) {
    pathInputs = { ...pathInputs, [key]: value }
  }

  function pathFields() {
    if (!cfg) return []
    return [
      {
        id: 'claude-dir',
        label: 'Claude 配置目录',
        inputKey: 'claudeDir',
        section: 'claude',
        key: 'path',
        placeholderLabel: '默认路径',
        placeholderPath: cfg.claudeDirDefault || cfg.claudeDir,
      },
      {
        id: 'claude-binary-path',
        label: 'Claude 二进制文件',
        inputKey: 'claudeBinary',
        section: 'binary',
        key: 'claude_path',
        placeholderLabel: cfg.claudeBinaryValid ? '自动检测' : '默认路径',
        placeholderPath: cfg.claudeBinaryPlaceholderPath || cfg.claudeBinaryManagedPath || cfg.claudeBinaryPath,
      },
      {
        id: 'claude-json-path',
        label: 'Claude JSON 配置文件（.claude.json）',
        inputKey: 'claudeJSON',
        section: 'claude',
        key: 'json_path',
        placeholderLabel: '默认路径',
        placeholderPath: cfg.claudeJSONPathDefault || cfg.claudeJSONPath,
      },
    ]
  }

  function pathPlaceholder(field) {
    return field.placeholderPath ? `${field.placeholderLabel}：${field.placeholderPath}` : '留空使用自动检测'
  }

  async function savePathField(field) {
    const input = document.getElementById(field.id)
    const value = (input?.value ?? pathInputs[field.inputKey] ?? '').trim()
    setPathInput(field.inputKey, value)
    await saveField(field.section, field.key, value)
    await loadConfig()
  }

  async function ensureExcludeLoaded() {
    if (excludeLoaded) return
    excludeLoaded = true
    await Promise.all([loadClaudeDirectories(), loadClaudeExcludeFiles()])
  }

  async function ensureEncryptionStatus() {
    if (encryptionLoaded) return
    encryptionLoaded = true
    try { encStatus = await GetEncryptionStatus() } catch (e) { encStatus = null }
  }

  async function loadClaudeDirectories() {
    try {
      claudeDirs = await GetClaudeDirectories()
    } catch (e) {
      claudeDirs = []
      error = e.message || String(e)
    }
  }

  async function loadClaudeExcludeFiles() {
    try {
      claudeFiles = await GetClaudeExcludeFiles()
    } catch (e) {
      claudeFiles = []
      error = e.message || String(e)
    }
  }

  async function excludeDirectory(dir) {
    await excludePattern(dir.pattern, 'directory')
  }

  async function excludeFile(file) {
    await excludePattern(file.pattern, 'file')
  }

  async function excludePattern(pattern, kind) {
    try {
      await AddExcludePattern(pattern)
      if (!excludeList.includes(pattern)) excludeList = [...excludeList, pattern]
      if (kind === 'directory') claudeDirs = claudeDirs.map(item => item.pattern === pattern ? { ...item, excluded: true } : item)
      if (kind === 'file') claudeFiles = claudeFiles.map(item => item.pattern === pattern ? { ...item, excluded: true } : item)
      showSaved()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function restorePattern(pattern) {
    try {
      await RemoveExcludePattern(pattern)
      excludeList = excludeList.filter(x => x !== pattern)
      claudeDirs = claudeDirs.map(item => item.pattern === pattern ? { ...item, excluded: false } : item)
      claudeFiles = claudeFiles.map(item => item.pattern === pattern ? { ...item, excluded: false } : item)
      showSaved()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  function isDirectoryPattern(pattern) {
    return String(pattern || '').endsWith('/') && !String(pattern || '').includes('*')
  }

  function isManagedFilePattern(pattern) {
    return claudeFiles.some(item => item.pattern === pattern)
  }

  function directoryLabel(pattern) {
    const dir = claudeDirs.find(item => item.pattern === pattern)
    return dir ? `${dir.name}/` : pattern
  }

  function fileLabel(pattern) {
    const file = claudeFiles.find(item => item.pattern === pattern)
    return file ? file.name : pattern
  }

  async function verifyKey() {
    verifyLoading = true; verifyResult = null
    try {
      verifyResult = await VerifyEncryptionKey()
    } catch (e) {
      verifyResult = { status: 'error', message: e.message || String(e) }
    }
    verifyLoading = false
  }

  function scheduleInputEncPreview() {
    clearTimeout(inputEncPreviewTimer)
    inputEncPreviewRequest += 1
    if (!inputEncPass) {
      inputEncPreview = null
      inputEncPreviewLoading = false
      return
    }
    const requestId = inputEncPreviewRequest
    const pass = inputEncPass
    inputEncPreviewLoading = true
    inputEncPreviewTimer = setTimeout(() => previewInputEncPass(pass, requestId), 600)
  }

  async function previewInputEncPass(pass, requestId) {
    try {
      const result = await PreviewEncryptionPassword(pass)
      if (requestId === inputEncPreviewRequest && inputEncPass === pass) inputEncPreview = result
    } catch (e) {
      if (requestId === inputEncPreviewRequest && inputEncPass === pass) inputEncPreview = { status: 'error', message: e.message || String(e) }
    }
    if (requestId === inputEncPreviewRequest && inputEncPass === pass) inputEncPreviewLoading = false
  }

  function scheduleOldEncPreview() {
    clearTimeout(oldEncPreviewTimer)
    oldEncPreviewRequest += 1
    if (!oldEncPass) {
      oldEncPreview = null
      oldEncPreviewLoading = false
      return
    }
    const requestId = oldEncPreviewRequest
    const pass = oldEncPass
    oldEncPreviewLoading = true
    oldEncPreviewTimer = setTimeout(() => previewOldEncPass(pass, requestId), 600)
  }

  async function previewOldEncPass(pass, requestId) {
    try {
      const result = await PreviewEncryptionPassword(pass)
      if (requestId === oldEncPreviewRequest && oldEncPass === pass) oldEncPreview = result
    } catch (e) {
      if (requestId === oldEncPreviewRequest && oldEncPass === pass) oldEncPreview = { status: 'error', message: e.message || String(e) }
    }
    if (requestId === oldEncPreviewRequest && oldEncPass === pass) oldEncPreviewLoading = false
  }

  async function saveEncryptionPassword() {
    if (!inputEncPass) { error = '请输入加密密码'; return }
    saveEncPassLoading = true; error = ''
    try {
      await SaveEncryptionPassword(inputEncPass)
      saved = '加密密码已保存'
      inputEncPass = ''; inputEncPreview = null
      encStatus = await GetEncryptionStatus()
      verifyResult = await VerifyEncryptionKey()
      clearTimeout(savedTimer); savedTimer = setTimeout(() => saved = '', 2000)
    } catch (e) {
      error = e.message || String(e)
    }
    saveEncPassLoading = false
  }

  async function changePassword() {
    if (!oldEncPass || !newEncPass || !confirmEncPass) { error = '请填写所有加密密码字段'; return }
    if (newEncPass !== confirmEncPass) { error = '新加密密码两次输入不一致'; return }
    if (newEncPass.length < 6) { error = '新加密密码至少 6 个字符'; return }
    changePassLoading = true; error = ''
    try {
      await ChangeEncryptionPassword(oldEncPass, newEncPass)
      saved = '加密密码已修改'
      oldEncPass = ''; oldEncPreview = null; newEncPass = ''; confirmEncPass = ''
      encStatus = await GetEncryptionStatus()
      verifyResult = await VerifyEncryptionKey()
      clearTimeout(savedTimer); savedTimer = setTimeout(() => saved = '', 2000)
    } catch (e) {
      error = e.message || String(e)
    }
    changePassLoading = false
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

  let savedTimer = null
  function showSaved() {
    clearTimeout(savedTimer)
    saved = '已保存'
    savedTimer = setTimeout(() => saved = '', 2000)
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
          <label class="label" for="webdav-url">WebDAV 服务地址</label>
          <div class="input-row">
            <input id="webdav-url" class="input" type="text" bind:value={webdavUrl} placeholder="https://your-server/dav/" />
            <button class="btn-sm" on:click={saveWebdavUrl}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label" for="webdav-user">用户名</label>
          <div class="input-row">
            <input id="webdav-user" class="input" type="text" bind:value={webdavUser} placeholder="用户名" />
            <button class="btn-sm" on:click={saveWebdavUser}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label" for="webdav-password">密码</label>
          <div class="input-row">
            <input id="webdav-password" class="input" type="password" bind:value={webdavPassNew} placeholder={cfg.webdav.hasPassword ? '••••••••（已保存）' : '输入密码'} />
            <button class="btn-sm" on:click={savePassword}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label" for="webdav-root">自定义 WebDAV 存储目录</label>
          <div class="input-row">
            <input id="webdav-root" class="input" type="text" bind:value={webdavRoot} placeholder="例如 cc-box，可留空" />
            <button class="btn-sm" on:click={saveWebdavRoot}>保存</button>
          </div>
          <div class="hint">可填写 cc-box、/cc-box/；留空时直接使用 WebDAV 服务地址当前目录。</div>
        </div>
        <div class="form-group">
          <label class="label" for="webdav-proxy">代理地址（可选）</label>
          <div class="input-row">
            <input id="webdav-proxy" class="input" type="text" bind:value={webdavProxyUrl} placeholder="例如 http://127.0.0.1:7890" />
            <button class="btn-sm" on:click={saveWebdavProxyUrl}>保存</button>
          </div>
          <div class="hint">仅 WebDAV 连接使用代理；留空使用系统默认代理设置</div>
        </div>
        <div>
          {#if webdavPathDirty}
            <div class="hint warn">当前 WebDAV 地址或存储目录有未保存修改，保存后下方路径会更新。</div>
          {/if}
          {#if cfg.webdav.baseUrl}
            <div class="path-lines">
              <div class="path-line">
                <span>当前生效路径</span>
                <code>{cfg.webdav.baseUrl}</code>
              </div>
              <div class="path-line check">
                <span>HEAD 检查路径</span>
                <code>{cfg.webdav.headUrl}</code>
              </div>
            </div>
          {:else}
            <div class="hint">保存 WebDAV 服务地址后会显示最终路径。</div>
          {/if}
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
        <div class="section-label-row">
          <span class="section-label">加密状态</span>
        </div>
        <div class="info-row">
          <span class="info-label">加密同步</span>
          <span class="status-pill" class:on={encryptionEnabled}>{encryptionEnabled ? '已启用' : '未启用'}</span>
        </div>
        <div class="info-row">
          <span class="info-label">加密算法</span>
          <span class="info-value font-mono">AES-256-GCM</span>
        </div>
        <div class="info-row">
          <span class="info-label">密码派生</span>
          <span class="info-value font-mono">Argon2id</span>
        </div>
        {#if encStatus && encStatus.hasKey}
          <div class="info-row">
            <span class="info-label">当前指纹</span>
            <span class="fingerprint font-mono">{encStatus.fingerprint}</span>
          </div>
          <div class="hint">指纹用于识别本机当前加密状态，不是密码。</div>
          <div class="form-actions">
            {#if verifyResult?.status === 'success'}
              <span class="test-result ok">{verifyResult.message}</span>
            {:else if verifyResult?.status === 'mismatch'}
              <span class="test-result err">{verifyResult.message}</span>
            {:else if verifyResult?.status === 'unverified'}
              <span class="test-result warn">{verifyResult.message}</span>
            {:else if verifyResult?.status === 'error'}
              <span class="test-result err">{verifyResult.message}</span>
            {/if}
            <button class="btn-sm" disabled={verifyLoading} on:click={verifyKey}>
              {verifyLoading ? '验证中...' : '验证加密密码'}
            </button>
          </div>
        {:else}
          <div class="empty-compact">未检测到本机加密密码</div>
        {/if}
      </div>

      {#if encStatus && encStatus.hasKey}
        <div class="card animate-fade-in">
          <div class="section-label-row">
            <span class="section-label">重新输入加密密码</span>
          </div>
          <div class="hint">当其他设备修改过加密密码后，在这里输入新的加密密码。</div>
          <div class="form-group">
            <label class="label" for="input-encryption-password">加密密码</label>
            <input id="input-encryption-password" class="input" type="password" bind:value={inputEncPass} placeholder="输入加密密码" on:input={scheduleInputEncPreview} />
            {#if inputEncPreviewLoading}
              <div class="fingerprint-preview pending">正在计算输入密码对应指纹...</div>
            {:else if inputEncPreview?.fingerprint}
              <div class="fingerprint-preview" class:ok={inputEncPreview.status === 'success'} class:err={inputEncPreview.status === 'mismatch' || inputEncPreview.status === 'error'}>
                <span>输入密码对应指纹</span>
                <code>{inputEncPreview.fingerprint}</code>
                {#if inputEncPreview.matchesCurrent}
                  <em>与当前指纹一致</em>
                {:else}
                  <em>{inputEncPreview.message}</em>
                {/if}
              </div>
            {:else if inputEncPreview?.message}
              <div class="fingerprint-preview err">{inputEncPreview.message}</div>
            {/if}
          </div>
          <div class="form-actions">
            <button class="btn-primary" disabled={saveEncPassLoading || !inputEncPass} on:click={saveEncryptionPassword}>
              {saveEncPassLoading ? '保存中...' : '保存加密密码'}
            </button>
          </div>
        </div>

        <div class="card animate-fade-in">
          <div class="section-label-row">
            <span class="section-label">修改加密密码</span>
          </div>
          <div class="form-group">
            <label class="label" for="old-encryption-password">当前加密密码</label>
            <input id="old-encryption-password" class="input" type="password" bind:value={oldEncPass} placeholder="输入当前加密密码" on:input={scheduleOldEncPreview} />
            {#if oldEncPreviewLoading}
              <div class="fingerprint-preview pending">正在计算当前加密密码对应指纹...</div>
            {:else if oldEncPreview?.fingerprint}
              <div class="fingerprint-preview" class:ok={oldEncPreview.status === 'success'} class:err={oldEncPreview.status === 'mismatch' || oldEncPreview.status === 'error'}>
                <span>当前加密密码对应指纹</span>
                <code>{oldEncPreview.fingerprint}</code>
                {#if oldEncPreview.matchesCurrent}
                  <em>与当前指纹一致</em>
                {:else}
                  <em>{oldEncPreview.message}</em>
                {/if}
              </div>
            {:else if oldEncPreview?.message}
              <div class="fingerprint-preview err">{oldEncPreview.message}</div>
            {/if}
          </div>
          <div class="form-group">
            <label class="label" for="new-encryption-password">新加密密码</label>
            <input id="new-encryption-password" class="input" type="password" bind:value={newEncPass} placeholder="至少 6 个字符" />
            <div class="hint">新加密密码会生成新的指纹，修改成功后会刷新当前指纹。</div>
          </div>
          <div class="form-group">
            <label class="label" for="confirm-encryption-password">确认新加密密码</label>
            <input id="confirm-encryption-password" class="input" type="password" bind:value={confirmEncPass} placeholder="再次输入新加密密码" />
          </div>
          <div class="form-actions">
            <button class="btn-primary" disabled={changePassLoading} on:click={changePassword}>
              {changePassLoading ? '修改中...' : '修改加密密码'}
            </button>
          </div>
          <div class="warn-box">
            修改加密密码将重新加密所有远程数据，其他设备需重新输入新的加密密码。
          </div>
        </div>
      {/if}
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
            <span class="info-label">自动配置 PATH</span>
            <span class="info-desc">默认关闭；开启后，后续可由 CC-Box 配置用户级 PATH</span>
          </div>
          <button class="toggle-btn" class:on={autoConfigurePath} on:click={toggleAutoConfigurePath}>
            <span class="toggle-knob"></span>
          </button>
        </div>
        <div class="toggle-row">
          <div class="toggle-info">
            <span class="info-label">GitHub 签名校验</span>
            <span class="info-desc">默认关闭；开启后安装时校验 SHASUMS256.txt.sig GPG 签名（可能因网络问题安装失败）</span>
          </div>
          <button class="toggle-btn" class:on={binaryVerifySignature} on:click={toggleVerifySignature}>
            <span class="toggle-knob"></span>
          </button>
        </div>
        <div class="form-group">
          <label class="label" for="chunk-mode">分块模式</label>
          <select id="chunk-mode" class="input select-input" bind:value={chunkMode} on:change={saveChunkMode}>
            <option value="auto">自动</option>
            <option value="always">始终分块</option>
            <option value="never">不分块</option>
          </select>
        </div>
        <div class="form-group">
          <label class="label" for="chunk-size-mb">分块大小 (MB)</label>
          <div class="input-row">
            <input id="chunk-size-mb" class="input" type="number" bind:value={chunkSizeMB} min="1" max="100" />
            <button class="btn-sm" on:click={saveChunkSize}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label" for="chunk-threshold-mb">分块阈值 (MB)</label>
          <div class="input-row">
            <input id="chunk-threshold-mb" class="input" type="number" bind:value={chunkThresholdMB} min="1" max="500" />
            <button class="btn-sm" on:click={saveChunkThreshold}>保存</button>
          </div>
          <div class="hint">超过此大小的文件在自动模式下将分块上传</div>
        </div>
      </div>
    {/if}

    <!-- 同步 -->
    {#if activeTab === 'sync'}
      <div class="card animate-fade-in">
        <div class="toggle-row">
          <div class="toggle-info">
            <span class="info-label">同步时包含 Claude binary</span>
            <span class="info-desc">开启后，普通 push / pull / sync 会同步当前平台 Claude binary 状态</span>
          </div>
          <button class="toggle-btn" class:on={binarySyncEnabled} on:click={toggleBinarySync}>
            <span class="toggle-knob"></span>
          </button>
        </div>
        <div class="form-group">
          <label class="label" for="snapshot-limit">快照保留数</label>
          <div class="input-row">
            <input id="snapshot-limit" class="input" type="number" bind:value={snapshotLimit} min="5" max="200" />
            <button class="btn-sm" on:click={saveSnapshotLimit}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label" for="conflict-strategy">冲突策略</label>
          <select id="conflict-strategy" class="input select-input" bind:value={conflictStrategy} on:change={saveConflictStrategy}>
            <option value="ask">询问</option>
            <option value="local">保留本地</option>
            <option value="remote">采用远程</option>
            <option value="merge">尝试合并</option>
          </select>
        </div>
        <div class="form-group">
          <label class="label" for="merge-retry-max">合并重试次数</label>
          <div class="input-row">
            <input id="merge-retry-max" class="input" type="number" bind:value={mergeRetryMax} min="1" max="10" />
            <button class="btn-sm" on:click={saveMergeRetry}>保存</button>
          </div>
        </div>
        <div class="form-group">
          <label class="label" for="auto-sync-interval">自动同步间隔</label>
          <select id="auto-sync-interval" class="input select-input" bind:value={autoSyncInterval} on:change={saveAutoSyncInterval}>
            <option value="">关闭</option>
            <option value="5m">每 5 分钟</option>
            <option value="15m">每 15 分钟</option>
            <option value="30m">每 30 分钟</option>
            <option value="60m">每 60 分钟</option>
          </select>
          <div class="hint">监听 {cfg.claudeDir} 变更并自动同步到云端</div>
        </div>
      </div>
    {/if}

    <!-- 路径 -->
    {#if activeTab === 'paths'}
      <div class="card animate-fade-in">
        {#each pathFields() as field}
          <div class="form-group">
            <label class="label" for={field.id}>{field.label}</label>
            <div class="input-row">
              <input
                id={field.id}
                class="input font-mono"
                type="text"
                value={pathInputs[field.inputKey]}
                placeholder={pathPlaceholder(field)}
                on:input={(e) => setPathInput(field.inputKey, e.currentTarget.value)}
              />
              <button class="btn-sm" on:click={() => savePathField(field)}>保存</button>
            </div>
            {#if field.inputKey === 'claudeBinary' && cfg.claudeBinaryShim}
              <div class="hint text-state-err">检测到脚本 shim，仅显示版本，不支持作为可上传二进制。</div>
            {/if}
            {#if field.inputKey === 'claudeBinary' && cfg.claudeBinaryError}
              <div class="hint text-state-err">{cfg.claudeBinaryError}</div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <!-- 排除规则 -->
    {#if activeTab === 'exclude'}
      <div class="card animate-fade-in">
        <div class="exclude-header">
          <div>
            <span class="info-label">同步目录</span>
            <div class="hint">只管理 {cfg.claudeDir} 下的一级目录，点击 × 可排除目录。</div>
          </div>
          <span class="text-xs text-txt-muted">{availableClaudeDirs.length} 个目录</span>
        </div>
        <div class="exclude-list">
          {#if availableClaudeDirs.length === 0}
            <div class="empty-compact">没有可同步的目录</div>
          {:else}
            {#each availableClaudeDirs as dir (dir.pattern)}
              <div class="exclude-row">
                <div class="exclude-left">
                  <span class="exclude-pattern font-mono">{dir.name}/</span>
                  <span class="exclude-path font-mono">{dir.path}</span>
                </div>
                <button class="exclude-action remove" title="排除目录" on:click={() => excludeDirectory(dir)}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                    <path d="M18 6L6 18M6 6l12 12"/>
                  </svg>
                </button>
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <div class="card animate-fade-in">
        <div class="exclude-header">
          <div>
            <span class="info-label">同步文件</span>
            <div class="hint">可单独管理 {cfg.claudeDir} 下的 settings.json，点击 × 可排除文件。</div>
          </div>
          <span class="text-xs text-txt-muted">{availableClaudeFiles.length} 个文件</span>
        </div>
        <div class="exclude-list">
          {#if availableClaudeFiles.length === 0}
            <div class="empty-compact">没有可同步的文件</div>
          {:else}
            {#each availableClaudeFiles as file (file.pattern)}
              <div class="exclude-row">
                <div class="exclude-left">
                  <span class="exclude-pattern font-mono">{file.name}</span>
                  <span class="exclude-path font-mono">{file.path}</span>
                </div>
                <button class="exclude-action remove" title="排除文件" on:click={() => excludeFile(file)}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                    <path d="M18 6L6 18M6 6l12 12"/>
                  </svg>
                </button>
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <div class="card animate-fade-in">
        <div class="exclude-header">
          <div>
            <span class="info-label">已排除目录</span>
            <div class="hint">点击 + 恢复同步。</div>
          </div>
          <span class="text-xs text-txt-muted">{excludedDirectoryPatterns.length} 条规则</span>
        </div>
        <div class="exclude-list">
          {#if excludedDirectoryPatterns.length === 0}
            <div class="empty-compact">暂无排除目录</div>
          {:else}
            {#each excludedDirectoryPatterns as pattern (pattern)}
              <div class="exclude-row">
                <div class="exclude-left">
                  <span class="exclude-pattern font-mono">{directoryLabel(pattern)}</span>
                  <span class="exclude-type" class:default={isDefaultPattern(pattern)} class:custom={!isDefaultPattern(pattern)}>
                    {isDefaultPattern(pattern) ? '默认' : '自定义'}
                  </span>
                </div>
                <button class="exclude-action add" title="恢复同步" on:click={() => restorePattern(pattern)}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                    <path d="M12 5v14M5 12h14"/>
                  </svg>
                </button>
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <div class="card animate-fade-in">
        <div class="exclude-header">
          <div>
            <span class="info-label">已排除文件</span>
            <div class="hint">点击 + 恢复同步；这里只管理 settings.json。</div>
          </div>
          <span class="text-xs text-txt-muted">{excludedFilePatterns.length} 条规则</span>
        </div>
        <div class="exclude-list">
          {#if excludedFilePatterns.length === 0}
            <div class="empty-compact">暂无排除文件</div>
          {:else}
            {#each excludedFilePatterns as pattern (pattern)}
              <div class="exclude-row">
                <div class="exclude-left">
                  <span class="exclude-pattern font-mono">{fileLabel(pattern)}</span>
                  <span class="exclude-type custom">自定义</span>
                </div>
                <button class="exclude-action add" title="恢复同步" on:click={() => restorePattern(pattern)}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                    <path d="M12 5v14M5 12h14"/>
                  </svg>
                </button>
              </div>
            {/each}
          {/if}
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
  .hint.warn { color: rgb(var(--state-warn)); opacity: 0.9; }

  .path-lines { margin-top: 6px; display: flex; flex-direction: column; gap: 3px; }
  .path-line { display: flex; align-items: flex-start; gap: 8px; font-size: 11px; }
  .path-line span { flex: 0 0 76px; color: rgb(var(--text-muted)); opacity: 0.75; }
  .path-line code {
    color: rgb(var(--text-secondary)); font-family: 'DM Mono', monospace;
    word-break: break-all;
  }
  .path-line.check code { color: rgb(var(--accent)); }

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
  .test-result.warn { color: rgb(var(--state-warn)); }
  .test-result.err { color: rgb(var(--state-err)); }

  .toggle-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: 12px 0; border-bottom: 1px solid rgba(46,45,51,0.4);
  }
  .toggle-row:last-of-type { border-bottom: none; }
  .toggle-info { display: flex; flex-direction: column; gap: 2px; }

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
  .info-desc { font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.7; }
  .info-value { font-size: 12px; color: rgb(var(--text-primary)); }
  .status-pill {
    font-size: 11px; font-family: 'DM Mono', monospace;
    color: rgb(var(--text-muted));
  }
  .status-pill.on { color: rgb(var(--state-ok)); }
  .fingerprint {
    font-size: 14px; font-weight: 600; letter-spacing: 0.1em;
    color: rgb(var(--accent));
  }
  .fingerprint-preview {
    margin-top: 8px; display: flex; flex-wrap: wrap; align-items: center; gap: 6px;
    font-size: 11px; color: rgb(var(--text-muted));
  }
  .fingerprint-preview code {
    font-family: 'DM Mono', monospace; letter-spacing: 0.08em;
    color: rgb(var(--text-secondary));
  }
  .fingerprint-preview em { width: 100%; font-style: normal; opacity: 0.75; }
  .fingerprint-preview.ok code { color: rgb(var(--state-ok)); }
  .fingerprint-preview.err { color: rgb(var(--state-err)); }
  .fingerprint-preview.pending { opacity: 0.75; }

  .warn-box {
    margin-top: 12px; padding: 10px 14px; border-radius: 6px;
    font-size: 11px; color: rgb(var(--state-warn));
    background: rgba(196,165,78,0.06); border: 1px solid rgba(196,165,78,0.12);
  }

  .exclude-header {
    display: flex; align-items: flex-start; justify-content: space-between;
    gap: 12px; margin-bottom: 10px;
  }
  .exclude-list { display: flex; flex-direction: column; }
  .exclude-row {
    display: flex; align-items: center; justify-content: space-between;
    gap: 12px; padding: 7px 0; border-bottom: 1px solid rgba(46,45,51,0.3);
  }
  .exclude-row:last-child { border-bottom: none; }
  .exclude-left { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
  .exclude-pattern { font-size: 12px; color: rgb(var(--text-primary)); flex-shrink: 0; }
  .exclude-path {
    font-size: 10px; color: rgb(var(--text-muted)); opacity: 0.55;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .exclude-type {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 6px; border-radius: 3px; flex-shrink: 0;
  }
  .exclude-type.default { color: rgb(var(--text-muted)); background: rgb(var(--surface-2)); }
  .exclude-type.custom { color: rgb(var(--accent)); background: rgba(196,112,78,0.08); }
  .exclude-action {
    width: 24px; height: 24px; border-radius: 4px;
    display: flex; align-items: center; justify-content: center;
    background: transparent; border: none; cursor: pointer;
    color: rgb(var(--text-muted)); opacity: 0.45; transition: all 0.2s;
    flex-shrink: 0;
  }
  .exclude-action:hover { opacity: 1; }
  .exclude-action.remove:hover { color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }
  .exclude-action.add:hover { color: rgb(var(--state-ok)); background: rgba(107,144,128,0.08); }

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
