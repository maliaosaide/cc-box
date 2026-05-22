<script>
  import { onMount } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { GetBinaryPage, SwitchBinaryVersion, UploadBinaryVersion, UploadCurrentBinary, GetBinaryStorage, DeleteLocalVersion, DeleteCloudBinaryVersion, RedetectClaudeBinary, BrowseFile, SetConfigField } from '../../wailsjs/go/main/App.js'

  export let active = false

  let activeTab = 'claude'
  let binData = null
  let storage = null
  let loading = true
  let error = ''
  let msg = ''
  let switching = ''
  let uploading = ''
  let uploadProgress = null
  let detecting = false
  let promptHidden = false

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

  onMount(async () => {
    await loadBinary()
    EventsOn('op:progress', (e) => {
      if (e.operation === 'binary-upload') uploadProgress = e
    })
    EventsOn('op:complete', (e) => {
      if (e?.operation !== 'binary-upload' || !uploadProgress) return
      if (e.status === 'error') {
        error = e.error || '上传失败'
      } else {
        msg = '上传完成'
        if (active) loadBinary()
      }
      uploadProgress = null
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

  function formatSize(b) {
    if (!b) return '-'
    if (b < 1024) return b + ' B'
    if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
    return (b / 1024 / 1024).toFixed(1) + ' MB'
  }

  function sourceLabel(source) {
    return ({ configured: '手动配置', environment: '环境变量', cache: '缓存', bin_dir: '二进制目录', path: 'PATH', common: '常见目录', not_found: '未找到' })[source] || source || '-'
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

  function upload(version) {
    msg = ''; error = ''
    uploading = version
    UploadBinaryVersion(version)
  }

  function uploadCurrent() {
    msg = ''; error = ''
    uploading = 'current'
    UploadCurrentBinary()
  }

  async function deleteVersion(version) {
    msg = ''; error = ''
    try {
      await DeleteLocalVersion(version)
      msg = `已删除本地版本 ${version}`
      await loadBinary()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function deleteCloud(version) {
    msg = ''; error = ''
    try {
      await DeleteCloudBinaryVersion(version)
      msg = `已删除云端版本 ${version}`
      await loadBinary()
    } catch (e) {
      error = e.message || String(e)
    }
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
          <span class="item-detail font-mono">{binData.currentVersion || '未安装'}</span>
        </div>
        <span class="version-tag" class:latest={binData.localExists} class:!latest={!binData.localExists}>
          {binData.localExists ? '已安装' : '未安装'}
        </span>
        <div class="item-actions">
          <button class="btn-sm btn-upload"
                  disabled={!binData.localExists || binData.binaryShim || currentUploaded || !!uploadProgress}
                  on:click={uploadCurrent}>
            {#if currentUploaded}已上传
            {:else if uploadProgress && uploading === 'current'}上传中...
            {:else}上传到云端{/if}
          </button>
        </div>
      </div>
      {#if binData.binaryPath}
        <div class="path-row font-mono">{binData.binaryPath}</div>
      {/if}
      <div class="path-meta">
        <span>来源: {sourceLabel(binData.binarySource)}</span>
      </div>
      {#if binData.binaryShim}
        <div class="path-warn">检测到脚本 shim，仅用于版本显示，不支持上传为二进制版本。</div>
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
          <span class="storage-label">云端</span>
          <span class="storage-value">{formatSize(storage.cloudTotal)} ({storage.cloudCount} 个)</span>
        </div>
      </div>
    {/if}

    {#if binData.allVersions && binData.allVersions.length > 0}
      <div class="card animate-fade-in stagger-2">
        <div class="section-label-row">
          <span class="section-label">所有版本</span>
          <span class="text-xs text-txt-muted">{binData.allVersions.length} 个</span>
        </div>
        <div class="item-list">
          {#each binData.allVersions as ver}
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
                {#if ver.isRemote}<span class="cloud-tag">云端</span>{/if}
              </div>
              <div class="item-actions">
                {#if !ver.isCurrent}
                  {#if ver.isLocal}
                    <button class="btn-sm"
                            disabled={switching === ver.version + '-local'}
                            on:click={() => switchTo(ver.version, 'local')}>
                      {switching === ver.version + '-local' ? '切换中...' : '切换'}
                    </button>
                  {/if}
                  {#if ver.isRemote && !ver.isLocal}
                    <button class="btn-sm"
                            disabled={switching === ver.version + '-remote'}
                            on:click={() => switchTo(ver.version, 'remote')}>
                      {switching === ver.version + '-remote' ? '下载中...' : '下载切换'}
                    </button>
                  {/if}
                {/if}
                {#if ver.isLocal && !ver.isRemote}
                  <button class="btn-sm btn-upload"
                          disabled={!!uploadProgress}
                          on:click={() => upload(ver.version)}>
                    {uploadProgress && uploading === ver.version ? '上传中...' : '上传'}
                  </button>
                {:else if ver.isLocal && ver.isRemote}
                  <button class="btn-sm btn-upload" disabled>
                    已上传
                  </button>
                {/if}
                {#if !ver.isCurrent}
                  {#if ver.isLocal}
                    <button class="btn-del-sm" on:click|stopPropagation={() => deleteVersion(ver.version)} title="删除本地">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
                        <path d="M18 6L6 18M6 6l12 12"/>
                      </svg>
                    </button>
                  {/if}
                  {#if ver.isRemote}
                    <button class="btn-del-sm" on:click|stopPropagation={() => deleteCloud(ver.version)} title="删除云端">
                      <svg viewBox="0 0 20 20" fill="currentColor" width="12" height="12">
                        <path d="M3.172 5.172a4 4 0 015.656 0L10 6.343l1.172-1.171a4 4 0 115.656 5.656L10 17.657l-6.828-6.829a4 4 0 010-5.656z"/>
                      </svg>
                    </button>
                  {/if}
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>
    {:else if binData.localVersions && binData.localVersions.length > 0}
      <!-- fallback: allVersions 为空但 localVersions 有数据 -->
      <div class="card animate-fade-in stagger-2">
        <div class="section-label-row">
          <span class="section-label">本地版本</span>
          <span class="text-xs text-txt-muted">{binData.localVersions.length} 个</span>
        </div>
        <div class="item-list">
          {#each binData.localVersions as ver}
            <div class="item-row">
              <div class="ver-dot" class:current={ver.isCurrent}></div>
              <div class="item-main">
                <span class="item-name font-mono">{ver.version}</span>
                <span class="item-detail">{formatSize(ver.size)}</span>
              </div>
              <div class="item-actions">
                {#if !ver.isCurrent}
                  <button class="btn-sm"
                          disabled={switching === ver.version + '-local'}
                          on:click={() => switchTo(ver.version, 'local')}>
                    {switching === ver.version + '-local' ? '切换中...' : '切换'}
                  </button>
                {/if}
                <button class="btn-sm btn-upload"
                        disabled={!!uploadProgress}
                        on:click={() => upload(ver.version)}>
                  上传
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="card animate-fade-in stagger-2">
        <div class="empty-compact">暂无版本记录</div>
      </div>
    {/if}
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
  .progress-pct { font-size: 11px; color: rgb(var(--accent)); }
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
