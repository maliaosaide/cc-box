<script>
  import { onMount } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { GetBinaryPage, SwitchBinaryVersion, UploadBinaryVersion, UploadCurrentBinary, GetBinaryStorage, DeleteBinaryVersion, RedetectClaudeBinary, BrowseFile, SetConfigField, GetGitHubBinaryReleases, RefreshGitHubBinaryReleases, InstallOfficialClaude, InstallGitHubClaude, CancelOperation } from '../../wailsjs/go/main/App.js'

  export let active = false
  export let refreshToken = 0

  let activeTab = 'claude'
  let versionTab = 'local'
  let binData = null
  let storage = null
  let loading = true
  let error = ''
  let msg = ''
  let switching = ''
  let uploading = ''
  let uploadOpId = null
  let uploadProgress = null
  let detecting = false
  let promptHidden = false
  let lastRefreshToken = 0
  let githubData = null
  let githubLimit = 30
  let githubRefreshing = false
  let githubOpId = null
  let githubInstallOpId = null
  let githubInstallingVersion = ''
  let officialInstallOpId = null
  let cancelledInstallOpIds = new Set()
  let externalProgress = null
  let githubError = ''
  let githubInitialized = false

  $: if (active && refreshToken !== lastRefreshToken && !uploadOpId) {
    lastRefreshToken = refreshToken
    loadBinary()
  }

  const tabs = [
    { id: 'claude', label: 'Claude', active: true },
    { id: 'codex', label: 'Codex', coming: true },
    { id: 'gemini', label: 'Gemini', coming: true },
  ]

  const placeholders = {
    codex: { name: 'OpenAI Codex CLI', desc: 'OpenAI 编码助手命令行工具' },
    gemini: { name: 'Google Gemini CLI', desc: 'Google Gemini 命令行工具' },
  }

  $: currentVersionEntry = binData && binData.allVersions
    ? binData.allVersions.find(v => v.version === binData.currentVersion)
    : null
  $: currentUploaded = !!(currentVersionEntry && currentVersionEntry.isRemote)
  $: localVersions = binData?.allVersions ? binData.allVersions.filter(v => v.isLocal) : []
  $: cloudVersions = binData?.allVersions ? binData.allVersions.filter(v => v.isRemote) : []
  $: visibleVersions = versionTab === 'local' ? localVersions : versionTab === 'cloud' ? cloudVersions : []
  $: githubVersions = githubData?.releases || []
  $: activeInstallOpId = githubInstallOpId || officialInstallOpId
  $: canCancelExternalInstall = !!activeInstallOpId && ['binary-github-install', 'binary-official-install'].includes(externalProgress?.operation)
  $: canLoadMoreGithub = githubVersions.length >= githubLimit
  $: versionTabs = [
    { id: 'local', label: '本地', count: localVersions.length },
    { id: 'cloud', label: 'WebDAV', count: cloudVersions.length },
    { id: 'github', label: 'GitHub', count: githubVersions.length },
    { id: 'official', label: '官方安装' },
  ]

  onMount(async () => {
    await loadBinary()
    EventsOn('op:progress', (e) => {
      if (e.operation === 'binary-upload' && (!uploadOpId || e.opId === uploadOpId)) uploadProgress = e
      if (['binary-github-refresh', 'binary-github-install', 'binary-official-install'].includes(e.operation)) externalProgress = e
    })
    EventsOn('op:complete', async (e) => {
      if (e?.operation === 'binary-upload' && (!uploadOpId || e.opId === uploadOpId)) {
        if (e.status === 'error') {
          error = e.error || '上传失败'
        } else {
          msg = '上传完成'
          if (active) loadBinary()
        }
        uploadProgress = null
        uploadOpId = null
        uploading = ''
      }
      if (e?.operation === 'binary-github-refresh' && (!githubOpId || e.opId === githubOpId)) {
        githubRefreshing = false
        githubOpId = null
        if (e.status === 'error') githubError = e.error || '刷新 GitHub Release 失败'
        await loadGitHubCache()
        externalProgress = null
      }
      if (e?.operation === 'binary-github-install' && (!githubInstallOpId || e.opId === githubInstallOpId)) {
        finishExternalInstall(e, 'GitHub Release 安装完成', 'GitHub 安装失败')
        githubInstallOpId = null
        githubInstallingVersion = ''
        await loadBinary()
      }
      if (e?.operation === 'binary-official-install' && (!officialInstallOpId || e.opId === officialInstallOpId)) {
        finishExternalInstall(e, '官方安装完成', '官方安装失败')
        officialInstallOpId = null
        await loadBinary()
      }
    })
    EventsOn('op:cancelled', (e) => {
      if (!['binary-github-install', 'binary-official-install'].includes(e?.operation)) return
      markInstallCancelled(e.opId)
      if (e.opId === activeInstallOpId) externalProgress = { ...(externalProgress || {}), operation: e.operation, opId: e.opId, message: '正在取消安装...', percent: externalProgress?.percent || 0 }
    })
    EventsOn('data:changed', async (e) => {
      if (e?.domain !== 'binary') return
      if (e.source === 'github-releases') await loadGitHubCache()
      else await loadBinary()
    })
  })

  async function loadBinary() {
    loading = true; error = ''
    try {
      const [page, stats] = await Promise.all([GetBinaryPage(), GetBinaryStorage()])
      binData = page
      storage = stats
    }
    catch (e) { error = e.message || String(e) }
    loading = false
  }

  async function loadGitHubCache() {
    try {
      githubData = await GetGitHubBinaryReleases(githubLimit)
    } catch (e) {
      githubError = e.message || String(e)
    }
  }

  async function refreshGitHub() {
    if (githubOpId) return
    githubError = ''
    githubRefreshing = true
    externalProgress = { operation: 'binary-github-refresh', message: '正在刷新 GitHub Release', percent: 0 }
    try {
      githubOpId = await RefreshGitHubBinaryReleases(githubLimit)
    } catch (e) {
      githubRefreshing = false
      githubOpId = null
      githubError = e.message || String(e)
      externalProgress = null
    }
  }

  async function loadMoreGitHub() {
    githubLimit += 30
    await refreshGitHub()
  }

  async function selectVersionTab(tab) {
    versionTab = tab
    if (tab === 'github' && !githubInitialized) {
      githubInitialized = true
      await loadGitHubCache()
      await refreshGitHub()
    }
  }

  function formatSize(b) {
    if (!b) return '-'
    if (b < 1024) return b + ' B'
    if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
    return (b / 1024 / 1024).toFixed(1) + ' MB'
  }

  function sourceLabel(source) {
    return ({ configured: '手动配置', environment: '环境变量', cache: '缓存', bin_dir: '二进制目录', path: 'PATH', common: '常见目录', github: 'GitHub Release', official: '官方安装', webdav: 'WebDAV', local: '本地版本', not_found: '未找到' })[source] || source || '-'
  }

  function commandStatusLabel(status) {
    return ({ activated: '已激活', installed_not_activated: '未激活', shadowed_by_other_binary: '被其他路径遮蔽', not_installed: '未安装' })[status] || status || '-'
  }

  function formatDate(value) {
    if (!value) return '-'
    return new Date(value).toLocaleDateString()
  }

  async function redetect() {
    detecting = true; msg = ''; error = ''
    try {
      await RedetectClaudeBinary()
      await loadBinary()
      msg = '已重新检测 Claude 二进制'
      promptHidden = false
    } catch (e) {
      error = e.message || String(e)
    }
    detecting = false
  }

  async function chooseBinary() {
    msg = ''; error = ''
    try {
      const file = await BrowseFile('选择 Claude 可执行文件')
      if (file) {
        await SetConfigField('binary', 'claude_path', file)
        await loadBinary()
        msg = '已设置 Claude 可执行文件'
        promptHidden = false
      }
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function switchTo(version, source) {
    switching = version + '-' + source
    msg = ''; error = ''
    try {
      await SwitchBinaryVersion(version, source)
      msg = `已切换到 ${version}`
      await loadBinary()
    } catch (e) {
      error = e.message || String(e)
    }
    switching = ''
  }

  async function upload(version) {
    if (uploadOpId) return
    msg = ''; error = ''
    uploading = version
    uploadProgress = { message: '准备上传...', percent: 0 }
    try {
      uploadOpId = await UploadBinaryVersion(version)
    } catch (e) {
      error = e.message || String(e)
      uploading = ''
      uploadOpId = null
      uploadProgress = null
    }
  }

  async function uploadCurrent() {
    if (uploadOpId) return
    msg = ''; error = ''
    uploading = 'current'
    uploadProgress = { message: '准备上传...', percent: 0 }
    try {
      uploadOpId = await UploadCurrentBinary()
    } catch (e) {
      error = e.message || String(e)
      uploading = ''
      uploadOpId = null
      uploadProgress = null
    }
  }

  function markInstallCancelled(opId) {
    cancelledInstallOpIds = new Set([...cancelledInstallOpIds, opId])
  }

  function finishExternalInstall(e, successText, errorText) {
    const wasCancelled = cancelledInstallOpIds.has(e.opId)
    if (wasCancelled) msg = '已取消安装'
    else if (e.status === 'error') error = e.error || errorText
    else msg = successText
    if (wasCancelled) {
      const next = new Set(cancelledInstallOpIds)
      next.delete(e.opId)
      cancelledInstallOpIds = next
    }
    externalProgress = null
  }

  async function cancelExternalInstall() {
    if (!activeInstallOpId) return
    markInstallCancelled(activeInstallOpId)
    externalProgress = { ...(externalProgress || {}), message: '正在取消安装...', percent: externalProgress?.percent || 0 }
    try {
      await CancelOperation(activeInstallOpId)
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function installOfficial() {
    if (officialInstallOpId) return
    if (!confirm('安装官方最新版可能覆盖当前本地 Claude binary。安装前会尽量备份现有真实二进制，确认继续？')) return
    msg = ''; error = ''
    externalProgress = { operation: 'binary-official-install', message: '准备执行官方安装', percent: 0 }
    try {
      officialInstallOpId = await InstallOfficialClaude()
    } catch (e) {
      officialInstallOpId = null
      externalProgress = null
      error = e.message || String(e)
    }
  }

  async function installGitHub(version) {
    if (githubInstallOpId) return
    if (!confirm(`安装 GitHub Release ${version} 会替换当前受管 Claude binary。安装前会备份现有目标文件，确认继续？`)) return
    msg = ''; error = ''
    githubInstallingVersion = version
    externalProgress = { operation: 'binary-github-install', message: `准备安装 ${version}`, percent: 0 }
    try {
      githubInstallOpId = await InstallGitHubClaude(version)
    } catch (e) {
      githubInstallOpId = null
      githubInstallingVersion = ''
      externalProgress = null
      error = e.message || String(e)
    }
  }

  async function deleteVersion(version) {
    if (!confirm(`删除 Claude ${version}？本地缓存和云端备份（如存在）都会删除。`)) return
    msg = ''; error = ''
    try {
      await DeleteBinaryVersion(version)
      msg = `已删除版本 ${version}`
      await loadBinary()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  function switchSource(ver) {
    return ver.isLocal ? 'local' : 'remote'
  }

  function switchLabel(ver) {
    if (ver.isLocal) return '切换到此版本'
    return '安装此版本'
  }

  function sourceEmptyText() {
    if (versionTab === 'local') return '暂无本地版本'
    if (versionTab === 'cloud') return '暂无 WebDAV 备份版本'
    return '暂无版本记录'
  }
</script>

<div class="bin-page">
  <h1 class="section-title animate-fade-in">二进制管理</h1>

  <div class="tabs-bar animate-fade-in">
    {#each tabs as tab}
      <button class="tab-btn" class:active={activeTab === tab.id}
              disabled={tab.coming}
              on:click={() => { if (tab.active) activeTab = tab.id }}>
        {tab.label}
        {#if tab.coming}
          <span class="coming-tag">即将支持</span>
        {/if}
      </button>
    {/each}
  </div>

  {#if msg}
    <div class="msg-bar animate-fade-in">
      <span>{msg}</span>
      <button class="link-btn" on:click={() => msg = ''}>关闭</button>
    </div>
  {/if}

  {#if error}
    <div class="error-bar animate-fade-in">
      <span>{error}</span>
      <button class="link-btn" on:click={() => error = ''}>关闭</button>
    </div>
  {/if}

  {#if uploadProgress}
    <div class="progress-section animate-fade-in">
      <div class="progress-header">
        <span class="progress-msg font-mono">{uploadProgress.message}</span>
        <span class="progress-pct font-mono">{Math.round(uploadProgress.percent)}%</span>
      </div>
      <div class="progress-bar">
        <div class="progress-bar-fill" style="width: {uploadProgress.percent}%"></div>
      </div>
    </div>
  {/if}

  {#if externalProgress}
    <div class="progress-section animate-fade-in">
      <div class="progress-header">
        <span class="progress-msg font-mono">{externalProgress.message}</span>
        <div class="progress-actions">
          <span class="progress-pct font-mono">{Math.round(externalProgress.percent || 0)}%</span>
          {#if canCancelExternalInstall}
            <button class="progress-cancel" on:click={cancelExternalInstall}>取消</button>
          {/if}
        </div>
      </div>
      <div class="progress-bar">
        <div class="progress-bar-fill" style="width: {externalProgress.percent || 0}%"></div>
      </div>
    </div>
  {/if}

  {#if activeTab !== 'claude'}
    <div class="card animate-fade-in">
      <div class="text-center py-16">
        <div class="empty-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="12" cy="12" r="10"/>
            <path d="M12 8v4m0 4h.01"/>
          </svg>
        </div>
        <p class="text-txt-primary text-sm font-medium mt-2">{placeholders[activeTab]?.name || activeTab}</p>
        <p class="text-txt-muted text-xs mt-1">{placeholders[activeTab]?.desc || '即将支持'}</p>
        <div class="mt-4">
          <span class="coming-badge">规划中</span>
        </div>
      </div>
    </div>
  {:else if loading}
    <div class="flex items-center justify-center h-64">
      <div class="loading-dot animate-gentle-pulse"></div>
    </div>
  {:else if binData}
    <div class="card animate-fade-in stagger-1">
      <div class="section-label-row">
        <span class="section-label">当前版本</span>
        <span class="font-mono text-xs text-txt-muted">{binData.platform}</span>
      </div>
      <div class="current-row">
        <div class="item-badge accent">C</div>
        <div class="item-main">
          <span class="item-name">claude</span>
          <span class="item-detail font-mono">{binData.currentVersion || (binData.localExists ? '版本待检测' : '未安装')}</span>
        </div>
        <span class="version-tag" class:latest={binData.localExists} class:!latest={!binData.localExists}>
          {binData.localExists ? '已安装' : '未安装'}
        </span>
        <div class="item-actions">
          <button class="btn-sm btn-upload"
                  disabled={!binData.localExists || binData.binaryShim || currentUploaded || !!uploadOpId}
                  on:click={uploadCurrent}>
            {#if currentUploaded}已上传
            {:else if uploadProgress && uploading === 'current'}上传中...
            {:else}上传当前本地版本{/if}
          </button>
        </div>
      </div>
      {#if binData.binaryPath}
        <div class="path-row font-mono">当前路径: {binData.binaryPath}</div>
      {/if}
      {#if binData.managedPath && binData.managedPath !== binData.binaryPath}
        <div class="path-row font-mono">安装目标: {binData.managedPath}</div>
      {/if}
      <div class="path-meta">
        <span>来源: {sourceLabel(binData.binarySource)}</span>
        <span>命令状态: {commandStatusLabel(binData.commandStatus?.status)}</span>
      </div>
      {#if binData.binaryShim}
        <div class="path-warn">当前 claude 命令入口看起来是脚本或 shim；继续安装会将安装目标替换为官方 native binary，不支持上传该 shim。</div>
      {/if}
      {#if !binData.localExists && !promptHidden}
        <div class="detect-panel">
          <div>
            <div class="detect-title">未找到 Claude 二进制</div>
            <div class="detect-msg">{binData.binaryError || '可以重新检测，或手动选择 Claude 可执行文件。'}</div>
          </div>
          <div class="detect-actions">
            <button class="btn-sm" disabled={detecting} on:click={redetect}>{detecting ? '检测中...' : '重新检测'}</button>
            <button class="btn-sm" on:click={chooseBinary}>手动选择</button>
            <button class="btn-sm btn-upload" on:click={() => promptHidden = true}>暂不处理</button>
          </div>
        </div>
      {/if}
    </div>

    {#if storage}
      <div class="storage-bar animate-fade-in">
        <div class="storage-item">
          <span class="storage-label">本地</span>
          <span class="storage-value">{formatSize(storage.localTotal)} ({storage.localCount} 个)</span>
        </div>
        <div class="storage-divider"></div>
        <div class="storage-item">
          <span class="storage-label">WebDAV</span>
          <span class="storage-value">{formatSize(storage.cloudTotal)} ({storage.cloudCount} 个)</span>
        </div>
      </div>
    {/if}

    <div class="card animate-fade-in stagger-2">
      <div class="section-label-row">
        <span class="section-label">版本来源</span>
        <span class="text-xs text-txt-muted">{binData.allVersions?.length || 0} 个已记录版本</span>
      </div>

      <div class="source-tabs">
        {#each versionTabs as tab}
          <button class="source-tab" class:active={versionTab === tab.id} on:click={() => selectVersionTab(tab.id)}>
            <span>{tab.label}</span>
            {#if tab.count !== undefined}
              <span class="source-count">{tab.count}</span>
            {/if}
          </button>
        {/each}
      </div>

      {#if versionTab === 'github'}
        <div class="source-head">
          <div>
            <p class="text-txt-primary text-sm font-medium">GitHub Releases</p>
            <p class="source-desc left">只显示当前平台可安装版本；安装时校验 SHASUMS256.txt 中的 SHA256，签名校验不阻塞安装。</p>
          </div>
          <button class="btn-sm" disabled={githubRefreshing} on:click={refreshGitHub}>{githubRefreshing ? '刷新中...' : '刷新版本列表'}</button>
        </div>
        {#if githubError}
          <div class="path-warn">{githubError}</div>
        {/if}
        {#if githubData && !githubData.supported}
          <div class="empty-compact">当前平台暂不支持 GitHub Release 安装：{githubData.platform}</div>
        {:else if githubVersions.length > 0}
          <div class="item-list">
            {#each githubVersions as rel (rel.version)}
              <div class="item-row">
                <div class="ver-dot"></div>
                <div class="item-main">
                  <span class="item-name font-mono">{rel.version}</span>
                  <span class="item-detail">{formatSize(rel.assetSize)} · {formatDate(rel.publishedAt)} · {rel.assetName} · {rel.signatureVerificationText || '安装时校验 SHA256'}</span>
                </div>
                <div class="item-tags"><span class="cloud-tag">GitHub</span></div>
                <div class="item-actions">
                  <button class="btn-sm" disabled={!!githubInstallOpId} on:click={() => installGitHub(rel.version)}>
                    {githubInstallOpId && githubInstallingVersion === rel.version ? '安装中...' : '安装此版本'}
                  </button>
                </div>
              </div>
            {/each}
          </div>
          <div class="source-footer">
            <span class="text-xs text-txt-muted">{githubData?.fromCache ? '已显示缓存' : '已刷新'}{githubData?.fetchedAt ? ` · ${formatDate(githubData.fetchedAt)}` : ''}</span>
            <button class="btn-sm" disabled={githubRefreshing} on:click={loadMoreGitHub}>{githubRefreshing ? '加载中...' : '继续加载'}</button>
          </div>
        {:else}
          <div class="empty-compact">暂无当前平台可安装的 GitHub Release 版本</div>
        {/if}
      {:else if versionTab === 'official'}
        <div class="source-placeholder">
          <div class="empty-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M12 3l7 4v5c0 4.2-2.7 7.5-7 9-4.3-1.5-7-4.8-7-9V7l7-4z"/>
              <path d="M9 12l2 2 4-5"/>
            </svg>
          </div>
          <p class="text-txt-primary text-sm font-medium">官方最新版</p>
          <p class="source-desc">官方安装只安装最新版；如需指定版本，请使用 GitHub Releases 或 WebDAV 备份版本。安装前会尽量备份现有真实二进制。</p>
          <button class="btn-sm btn-upload mt-4" disabled={!!officialInstallOpId} on:click={installOfficial}>{officialInstallOpId ? '安装中...' : '一键安装官方最新版'}</button>
        </div>
      {:else if visibleVersions.length > 0}
        <div class="item-list">
          {#each visibleVersions as ver (ver.version)}
            <div class="item-row">
              <div class="ver-dot" class:current={ver.isCurrent}></div>
              <div class="item-main">
                <span class="item-name font-mono">{ver.version}</span>
                <span class="item-detail">
                  {formatSize(ver.size)}
                  {#if ver.uploadedBy || ver.uploadedAt} · {ver.uploadedBy || '-'} · {ver.uploadedAt}{/if}
                </span>
              </div>
              <div class="item-tags">
                {#if ver.isLocal}<span class="loc-tag">本地</span>{/if}
                {#if ver.isRemote}<span class="cloud-tag">WebDAV</span>{/if}
              </div>
              <div class="item-actions">
                {#if ver.isCurrent}
                  <span class="current-pill">正在使用</span>
                {:else}
                  <button class="btn-sm"
                          disabled={switching === ver.version + '-' + switchSource(ver)}
                          on:click={() => switchTo(ver.version, switchSource(ver))}>
                    {switching === ver.version + '-' + switchSource(ver) ? (switchSource(ver) === 'remote' ? '下载中...' : '切换中...') : switchLabel(ver)}
                  </button>
                {/if}
                {#if ver.isLocal && !ver.isRemote}
                  <button class="btn-sm btn-upload"
                          disabled={!!uploadOpId}
                          on:click={() => upload(ver.version)}>
                    {uploadProgress && uploading === ver.version ? '上传中...' : '上传备份'}
                  </button>
                {:else if ver.isLocal && ver.isRemote && versionTab === 'local'}
                  <button class="btn-sm btn-upload" disabled>
                    已备份
                  </button>
                {/if}
                {#if !ver.isCurrent}
                  <button class="btn-del-sm" on:click|stopPropagation={() => deleteVersion(ver.version)} title="删除">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
                      <path d="M3 6h18"/>
                      <path d="M8 6V4h8v2"/>
                      <path d="M6 6l1 15h10l1-15"/>
                      <path d="M10 10v7M14 10v7"/>
                    </svg>
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <div class="empty-compact">{sourceEmptyText()}</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .bin-page { display: flex; flex-direction: column; gap: 12px; }

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
  .tab-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .coming-tag {
    font-size: 9px; padding: 1px 5px; border-radius: 3px;
    background: rgba(196,112,78,0.08); color: rgb(var(--accent));
    margin-left: 4px; font-family: 'DM Mono', monospace;
  }
  .coming-badge {
    display: inline-block; font-size: 11px; font-family: 'DM Mono', monospace;
    padding: 3px 10px; border-radius: 4px;
    background: rgba(196,112,78,0.06); color: rgb(var(--accent));
    border: 1px solid rgba(196,112,78,0.15);
  }

  .source-tabs {
    display: grid; grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 4px; margin-bottom: 10px;
  }
  .source-tab {
    display: flex; align-items: center; justify-content: center; gap: 5px;
    padding: 6px 8px; border-radius: 6px;
    font-size: 11px; color: rgb(var(--text-muted));
    background: rgb(var(--surface-1)); border: 1px solid rgb(var(--border));
    cursor: pointer; transition: all 0.2s;
  }
  .source-tab:hover { color: rgb(var(--text-secondary)); border-color: rgba(196,112,78,0.2); }
  .source-tab.active { color: rgb(var(--accent)); background: rgba(196,112,78,0.08); border-color: rgba(196,112,78,0.2); }
  .source-count {
    min-width: 16px; height: 16px; padding: 0 4px; border-radius: 8px;
    display: inline-flex; align-items: center; justify-content: center;
    font-size: 9px; font-family: 'DM Mono', monospace;
    color: rgb(var(--text-muted)); background: rgb(var(--surface-2));
  }
  .source-placeholder { text-align: center; padding: 28px 18px 24px; }
  .source-desc { margin: 6px auto 0; max-width: 360px; font-size: 11px; line-height: 1.6; color: rgb(var(--text-muted)); opacity: 0.7; }
  .source-desc.left { margin-left: 0; margin-right: 0; max-width: 520px; }
  .source-head, .source-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
  .source-footer { margin-top: 10px; margin-bottom: 0; }

  .msg-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(107,144,128,0.08); border: 1px solid rgba(107,144,128,0.15);
    font-size: 12px; color: rgb(var(--state-ok));
  }
  .error-bar {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-radius: 6px;
    background: rgba(184,92,92,0.08); border: 1px solid rgba(184,92,92,0.15);
    font-size: 12px; color: rgb(var(--state-err));
  }

  .progress-section {
    padding: 10px 14px; border-radius: 6px;
    background: rgb(var(--surface-1)); border: 1px solid rgb(var(--border));
  }
  .progress-header {
    display: flex; justify-content: space-between; margin-bottom: 6px;
  }
  .progress-msg { font-size: 11px; color: rgb(var(--text-muted)); }
  .progress-actions { display: flex; align-items: center; gap: 8px; }
  .progress-pct { font-size: 11px; color: rgb(var(--accent)); }
  .progress-cancel {
    font-size: 11px; color: rgb(var(--state-err)); background: transparent;
    border: none; cursor: pointer; padding: 0; font-family: inherit;
  }
  .progress-bar {
    height: 4px; border-radius: 2px; background: rgb(var(--surface-2)); overflow: hidden;
  }
  .progress-bar-fill {
    height: 100%; border-radius: 2px;
    background: linear-gradient(90deg, rgb(var(--accent)), rgba(196,112,78,0.6));
    transition: width 0.3s;
  }

  .section-label-row {
    display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px;
  }
  .section-label {
    font-size: 11px; font-weight: 600; text-transform: uppercase;
    letter-spacing: 0.05em; color: rgb(var(--text-muted));
  }

  .current-row {
    display: flex; align-items: center; gap: 10px; padding: 6px 0;
  }
  .item-badge {
    width: 28px; height: 28px; border-radius: 7px;
    display: flex; align-items: center; justify-content: center;
    font-size: 11px; font-family: 'Bricolage Grotesque', sans-serif;
    font-weight: 700; flex-shrink: 0;
  }
  .item-badge.accent { background: rgba(196,112,78,0.08); color: rgb(var(--accent)); }
  .item-main { flex: 1; min-width: 0; display: flex; align-items: baseline; gap: 8px; }
  .item-name { font-size: 12px; color: rgb(var(--text-primary)); }
  .item-detail { font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.7; }
  .item-actions { display: flex; gap: 4px; flex-shrink: 0; }
  .item-tags { display: flex; gap: 3px; flex-shrink: 0; }
  .loc-tag, .cloud-tag {
    font-size: 9px; padding: 1px 5px; border-radius: 3px;
    font-family: 'DM Mono', monospace;
  }
  .loc-tag {
    background: rgb(var(--surface-2)); color: rgb(var(--text-secondary));
  }
  .cloud-tag {
    background: rgba(196,112,78,0.08); color: rgb(var(--accent));
  }
  .version-tag {
    font-size: 10px; font-family: 'DM Mono', monospace;
    padding: 2px 7px; border-radius: 3px;
    color: rgb(var(--text-muted)); background: rgb(var(--surface-2));
  }
  .version-tag.latest { background: rgba(107,144,128,0.1); color: rgb(var(--state-ok)); }
  .current-pill {
    font-size: 10px; font-family: 'DM Mono', monospace;
    color: rgb(var(--state-ok)); padding: 3px 6px;
    border-radius: 4px; background: rgba(107,144,128,0.08);
  }

  .path-row {
    font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.5;
    padding: 4px 0 0 38px;
  }
  .path-meta {
    display: flex; flex-wrap: wrap; gap: 10px;
    padding: 4px 0 0 38px;
    font-size: 10px; color: rgb(var(--text-muted)); opacity: 0.65;
  }
  .path-warn {
    margin: 8px 0 0 38px; padding: 6px 8px; border-radius: 5px;
    font-size: 11px;
    color: rgb(var(--state-err));
    background: rgba(184,92,92,0.08); border: 1px solid rgba(184,92,92,0.15);
  }
  .detect-panel {
    display: flex; justify-content: space-between; align-items: center; gap: 12px;
    margin-top: 10px; padding: 10px 12px; border-radius: 7px;
    background: rgb(var(--surface-1)); border: 1px solid rgb(var(--border));
  }
  .detect-title { font-size: 12px; color: rgb(var(--text-primary)); font-weight: 600; }
  .detect-msg { margin-top: 2px; font-size: 11px; color: rgb(var(--text-muted)); opacity: 0.75; }
  .detect-actions { display: flex; gap: 6px; flex-shrink: 0; }

  .item-list { display: flex; flex-direction: column; }
  .item-row {
    display: flex; align-items: center; gap: 10px; padding: 7px 0;
    border-bottom: 1px solid rgba(46,45,51,0.4);
  }
  .item-row:last-child { border-bottom: none; }

  .ver-dot {
    width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
    background: rgb(var(--text-muted)); opacity: 0.3;
  }
  .ver-dot.current { background: rgb(var(--state-ok)); opacity: 1; }

  .btn-sm {
    font-size: 10px; font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    color: rgb(var(--accent));
    background: rgba(196,112,78,0.08);
    border: 1px solid rgba(196,112,78,0.15);
    padding: 3px 10px; border-radius: 4px; cursor: pointer;
    transition: all 0.2s; white-space: nowrap;
  }
  .btn-sm:hover { background: rgba(196,112,78,0.15); }
  .btn-sm:disabled { opacity: 0.4; cursor: not-allowed; }
  .btn-sm.btn-upload {
    color: rgb(var(--text-secondary));
    background: rgb(var(--surface-2));
    border-color: rgb(var(--border));
  }
  .btn-sm.btn-upload:hover { color: rgb(var(--accent)); background: rgba(196,112,78,0.08); border-color: rgba(196,112,78,0.15); }

  .btn-del-sm {
    width: 22px; height: 22px; border-radius: 4px;
    display: flex; align-items: center; justify-content: center;
    background: transparent; border: none; cursor: pointer;
    color: rgb(var(--text-muted)); opacity: 0.3; transition: all 0.2s;
  }
  .btn-del-sm:hover { opacity: 1; color: rgb(var(--state-err)); background: rgba(184,92,92,0.08); }

  .storage-bar {
    display: flex; align-items: center; gap: 12px;
    padding: 8px 14px; border-radius: 6px;
    background: rgb(var(--surface-1)); border: 1px solid rgb(var(--border));
  }
  .storage-item { display: flex; align-items: center; gap: 6px; }
  .storage-label { font-size: 10px; color: rgb(var(--text-muted)); font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; }
  .storage-value { font-size: 11px; color: rgb(var(--text-secondary)); font-family: 'DM Mono', monospace; }
  .storage-divider { width: 1px; height: 14px; background: rgb(var(--border)); }

  .link-btn {
    font-size: 11px; color: rgb(var(--text-muted));
    background: none; border: none; cursor: pointer; transition: color 0.2s;
  }
  .link-btn:hover { color: rgb(var(--accent)); }

  .empty-compact { text-align: center; padding: 14px 0; color: rgb(var(--text-muted)); font-size: 12px; opacity: 0.6; }
  .empty-icon { width: 48px; height: 48px; margin: 0 auto 12px; color: rgb(var(--text-muted)); opacity: 0.4; }
  .empty-icon svg { width: 100%; height: 100%; }
  .loading-dot { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--accent)); }
</style>
