<script>
  import { createEventDispatcher, onMount } from 'svelte'
  import {
    GetAppInfo, TestWebDAVConnection, InitNewDevice, InitJoinExisting, PreviewSetupEncryptionPassword
  } from '../../wailsjs/go/main/App.js'

  const dispatch = createEventDispatcher()

  let step = 0
  let mode = ''
  let webdav = { url: '', username: '', password: '', root: 'cc-box' }
  let preset = ''
  let testing = false
  let testResult = null
  let deviceName = ''
  let password = ''
  let confirmPassword = ''
  let passwordPreview = null
  let passwordPreviewLoading = false
  let passwordPreviewTimer = null
  let passwordPreviewRequest = 0
  let submitting = false
  let errorMsg = ''
  $: progressStep = step === 3 ? 2 : step

  const presets = {
    jianguoyun: { label: '坚果云', url: 'https://dav.jianguoyun.com/dav/', root: 'cc-box' },
    nextcloud: { label: 'NextCloud', url: '', root: 'cc-box' },
    synology: { label: 'Synology', url: '', root: 'cc-box' },
    alist: { label: 'Alist', url: '', root: 'dav/cc-box' },
  }

  onMount(async () => {
    try {
      const info = await GetAppInfo()
      deviceName = info.platform || 'unknown'
    } catch (e) { /* ignore */ }
  })

  function selectMode(m) {
    mode = m
    step = 1
    errorMsg = ''
    password = ''
    confirmPassword = ''
    passwordPreview = null
    passwordPreviewLoading = false
    clearTimeout(passwordPreviewTimer)
  }

  function backToModeSelect() {
    step = 0
    mode = ''
    errorMsg = ''
    testResult = null
    passwordPreview = null
    passwordPreviewLoading = false
    clearTimeout(passwordPreviewTimer)
  }

  function backToConnection() {
    step = 1
    errorMsg = ''
    passwordPreview = null
    passwordPreviewLoading = false
    clearTimeout(passwordPreviewTimer)
  }

  function formatError(e, fallback) {
    if (typeof e === 'string') return e
    return e?.message || e?.error || String(e || fallback)
  }

  function applyPreset(key) {
    preset = key
    const p = presets[key]
    if (p) { webdav.url = p.url; webdav.root = p.root }
  }

  async function testConnection() {
    testing = true; testResult = null
    try {
      await TestWebDAVConnection(webdav.url, webdav.username, webdav.password, webdav.root)
      testResult = { ok: true, msg: '连接成功' }
    } catch (e) {
      testResult = { ok: false, msg: e.message || '连接失败' }
    }
    testing = false
  }

  function goNext() { step = mode === 'new' ? 2 : 3; errorMsg = ''; passwordPreview = null }

  function schedulePasswordPreview() {
    clearTimeout(passwordPreviewTimer)
    passwordPreviewRequest += 1
    if (mode !== 'join' || !password) {
      passwordPreview = null
      passwordPreviewLoading = false
      return
    }
    const requestId = passwordPreviewRequest
    const request = { url: webdav.url, username: webdav.username, webdavPassword: webdav.password, root: webdav.root, password }
    passwordPreviewLoading = true
    passwordPreviewTimer = setTimeout(() => previewPassword(request, requestId), 600)
  }

  async function previewPassword(request, requestId) {
    try {
      const result = await PreviewSetupEncryptionPassword(request.url, request.username, request.webdavPassword, request.root, request.password)
      if (isCurrentPasswordPreview(request, requestId)) passwordPreview = result
    } catch (e) {
      if (isCurrentPasswordPreview(request, requestId)) passwordPreview = { status: 'error', message: e.message || String(e) }
    }
    if (isCurrentPasswordPreview(request, requestId)) passwordPreviewLoading = false
  }

  function isCurrentPasswordPreview(request, requestId) {
    return requestId === passwordPreviewRequest && mode === 'join' && password === request.password &&
      webdav.url === request.url && webdav.username === request.username && webdav.password === request.webdavPassword && webdav.root === request.root
  }

  function skipSetup() {
    dispatch('complete', { skipped: true })
  }

  async function submit() {
    if (!password) { errorMsg = '请输入加密密码'; return }
    if (mode === 'new' && password !== confirmPassword) { errorMsg = '两次加密密码不一致'; return }
    submitting = true; errorMsg = ''
    try {
      if (mode === 'new') {
        await InitNewDevice(webdav.url, webdav.username, webdav.password, webdav.root, password, deviceName)
      } else {
        await InitJoinExisting(webdav.url, webdav.username, webdav.password, webdav.root, password, deviceName)
      }
      dispatch('complete')
    } catch (e) { errorMsg = formatError(e, '初始化失败') }
    submitting = false
  }
</script>

<div class="onboard-wrap">
  <div class="onboard-bg"></div>

  <div class="onboard-content">
    {#if step === 0}
      <div class="text-center animate-fade-in">
        <div class="onboard-logo">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.2">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>

        <h1 class="font-display text-4xl font-bold text-txt-primary tracking-tight mb-2">CC-Box</h1>
        <p class="text-txt-secondary font-body text-sm mb-12">Claude Code 跨设备配置同步</p>

        <div class="space-y-3 max-w-sm mx-auto">
          <button class="skip-setup-btn animate-fade-in" on:click={skipSetup}>跳过，先进入主界面</button>

          <button class="choice-card animate-fade-in stagger-1" on:click={() => selectMode('new')}>
            <div class="choice-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 5v14m-7-7h14"/>
              </svg>
            </div>
            <div class="text-left">
              <div class="text-txt-primary font-medium text-sm">新建设备</div>
              <div class="text-txt-muted text-xs mt-0.5">首次使用，创建新的同步仓库</div>
            </div>
          </button>

          <button class="choice-card animate-fade-in stagger-2" on:click={() => selectMode('join')}>
            <div class="choice-icon choice-icon-blue">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M16 21v-2a4 4 0 00-4-4H5m0 0l3 3m-3-3l3-3M8 3v2a4 4 0 004 4h7m0 0l-3-3m3 3l-3 3"/>
              </svg>
            </div>
            <div class="text-left">
              <div class="text-txt-primary font-medium text-sm">加入已有同步组</div>
              <div class="text-txt-muted text-xs mt-0.5">连接已有同步组，加入后再选择恢复方式</div>
            </div>
          </button>
        </div>
      </div>

    {:else if step === 1}
      <div class="animate-fade-in">
        <button class="back-btn" on:click={backToModeSelect}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M15 18l-6-6 6-6"/>
          </svg>
          返回
        </button>

        <h2 class="section-title mb-1">连接 WebDAV 存储</h2>
        <p class="text-txt-muted text-sm mb-8">配置云端同步的存储后端</p>

        <div class="space-y-5">
          <div>
            <label class="label" for="preset">常用预设</label>
            <div class="flex flex-wrap gap-2">
              {#each Object.entries(presets) as [key, p]}
                <button class="btn-preset" class:active={preset === key}
                        on:click={() => applyPreset(key)}>
                  {p.label}
                </button>
              {/each}
              <button class="btn-preset" class:active={preset === 'custom'}
                      on:click={() => preset = 'custom'}>自定义</button>
            </div>
          </div>

          <div>
            <label class="label" for="url">服务地址</label>
            <input id="url" class="input" type="text" placeholder="https://dav.example.com/dav/"
                   bind:value={webdav.url} />
          </div>
          <div>
            <label class="label" for="user">用户名</label>
            <input id="user" class="input" type="text" placeholder="user@example.com"
                   bind:value={webdav.username} />
          </div>
          <div>
            <label class="label" for="pass">WebDAV 密码</label>
            <input id="pass" class="input" type="password" placeholder="应用密码"
                   bind:value={webdav.password} />
          </div>
          <div>
            <label class="label" for="root">自定义 WebDAV 存储目录</label>
            <input id="root" class="input" type="text" placeholder="例如 cc-box，可留空"
                   bind:value={webdav.root} />
          </div>

          {#if testResult}
            <div class="test-result"
                 class:test-ok={testResult.ok}
                 class:test-err={!testResult.ok}>
              <svg viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4">
                {#if testResult.ok}
                  <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm-1 14.25l-3.5-3.5 1.06-1.06L11 13.12l4.44-4.43 1.06 1.06L11 16.25z"/>
                {:else}
                  <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm3.54 12.46l-1.06 1.06L12 13.06l-2.48 2.46-1.06-1.06L10.94 12 8.46 9.54l1.06-1.06L12 10.94l2.48-2.48 1.06 1.06L13.06 12l2.48 2.46z"/>
                {/if}
              </svg>
              <span class="font-mono text-sm">{testResult.msg}</span>
            </div>
          {/if}

          <div class="flex justify-end gap-3 pt-1">
            <button class="btn-ghost" on:click={testConnection} disabled={testing}>
              {testing ? '测试中...' : '测试连接'}
            </button>
            <button class="btn-primary" on:click={goNext}
                    disabled={!webdav.url || !webdav.username || !webdav.password}>
              下一步
              <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M9 18l6-6-6-6"/>
              </svg>
            </button>
          </div>
        </div>
      </div>

    {:else if step === 2 || step === 3}
      <div class="animate-fade-in">
        <div class="nav-row">
          <button class="back-btn" on:click={backToConnection}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M15 18l-6-6 6-6"/>
            </svg>
            返回
          </button>
          <button class="switch-mode-btn" on:click={backToModeSelect}>更换初始化方式</button>
        </div>

        <h2 class="section-title mb-1">
          {step === 2 ? '设置加密密码' : '输入已有加密密码'}
        </h2>
        <p class="text-txt-muted text-sm mb-8">
          {step === 2
            ? '加密密码将用于端到端加密，所有设备需使用相同加密密码。'
            : '加密密码将用于验证云端数据。加入后可在主页面选择恢复方式。'}
        </p>

        <div class="space-y-5">
          <div>
            <label class="label" for="password">加密密码</label>
            <input id="password" class="input" type="password"
                   placeholder={step === 2 ? '设置加密密码' : '输入已有加密密码'}
                   bind:value={password}
                   on:input={schedulePasswordPreview} />
            {#if step === 2}
              <div class="hint">创建同步组后会生成当前加密指纹，可在设置页查看。</div>
            {:else if passwordPreviewLoading}
              <div class="fingerprint-preview pending">正在计算输入密码对应指纹...</div>
            {:else if passwordPreview?.fingerprint}
              <div class="fingerprint-preview" class:ok={passwordPreview.status === 'success'} class:err={passwordPreview.status === 'mismatch' || passwordPreview.status === 'error'}>
                <span>输入密码对应指纹</span>
                <code>{passwordPreview.fingerprint}</code>
                <em>{passwordPreview.message}</em>
              </div>
            {:else if passwordPreview?.message}
              <div class="fingerprint-preview err">{passwordPreview.message}</div>
            {/if}
          </div>

          {#if step === 2}
            <div>
              <label class="label" for="confirm">确认加密密码</label>
              <input id="confirm" class="input" type="password"
                     placeholder="再次输入加密密码"
                     bind:value={confirmPassword} />
            </div>
          {/if}

          <div>
            <label class="label" for="devicename">设备名称</label>
            <input id="devicename" class="input" type="text" bind:value={deviceName} />
          </div>

          {#if errorMsg}
            <div class="error-msg">{errorMsg}</div>
          {/if}

          {#if step === 3}
            <div class="info-box">
              <svg class="w-4 h-4 flex-shrink-0 mt-0.5" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 14a1.5 1.5 0 110-3 1.5 1.5 0 010 3zm1-5.5h-2V7h2v3.5z"/>
              </svg>
              <span>完成后不会立即覆盖本机配置。你可以在主页面选择拉取最新配置、恢复历史快照，或到二进制页面恢复 Claude binary。</span>
            </div>
          {/if}

          <div class="flex justify-end gap-3 pt-1">
            <button class="btn-primary" on:click={submit}
                    disabled={!password || (step === 2 && password !== confirmPassword) || submitting}>
              {submitting ? '处理中...' : (step === 3 ? '加入同步组' : '完成')}
              {#if !submitting}
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M5 12h14M12 5l7 7-7 7"/>
                </svg>
              {/if}
            </button>
          </div>
        </div>
      </div>
    {/if}

    <div class="flex items-center justify-center gap-2 mt-10">
      {#each [0, 1, 2] as i}
        <div class="step-dot" class:done={progressStep >= i} class:current={progressStep === i}></div>
      {/each}
    </div>
  </div>
</div>

<style>
  .onboard-wrap {
    height: 100%;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    position: relative;
    overflow-y: auto;
    padding: 32px 0;
    background: rgb(var(--surface-0));
  }

  .onboard-bg {
    position: absolute;
    inset: 0;
    opacity: 0.015;
    background-image:
      radial-gradient(ellipse at 25% 35%, rgb(var(--accent)) 0%, transparent 60%),
      radial-gradient(ellipse at 75% 65%, rgb(var(--state-sync)) 0%, transparent 60%);
  }

  .onboard-content {
    position: relative;
    width: 100%;
    max-width: 440px;
    padding: 0 32px;
    margin: auto 0;
  }

  .onboard-logo {
    width: 64px;
    height: 64px;
    border-radius: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 20px;
    background: rgba(196,112,78,0.08);
    border: 1px solid rgba(196,112,78,0.15);
    color: rgb(var(--accent));
  }
  .onboard-logo svg { width: 32px; height: 32px; }

  .skip-setup-btn {
    width: 100%;
    padding: 11px 14px;
    border-radius: 10px;
    border: 1px dashed rgb(var(--border));
    background: transparent;
    color: rgb(var(--text-muted));
    cursor: pointer;
    font-size: 13px;
    font-family: inherit;
  }
  .skip-setup-btn:hover { color: rgb(var(--text-secondary)); background: rgb(var(--surface-1)); }

  .choice-card {
    width: 100%;
    padding: 16px 18px;
    border-radius: 12px;
    border: 1px solid rgb(var(--border));
    background: rgb(var(--surface-1));
    display: flex;
    align-items: center;
    gap: 14px;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.22, 1, 0.36, 1);
    text-align: left;
    color: inherit;
    font-family: inherit;
    font-size: inherit;
  }
  .choice-card:hover {
    border-color: rgba(196,112,78,0.35);
    background: rgb(var(--surface-2));
    box-shadow: 0 4px 20px rgba(196,112,78,0.06);
    transform: translateY(-1px);
  }

  .choice-icon {
    width: 40px;
    height: 40px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    background: rgba(196,112,78,0.08);
    color: rgb(var(--accent));
  }
  .choice-icon svg { width: 20px; height: 20px; }
  .choice-icon-blue {
    background: rgba(91,127,165,0.08);
    color: rgb(var(--state-sync));
  }

  .nav-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  .back-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-bottom: 16px;
    font-size: 13px;
    color: rgb(var(--text-muted));
    background: none;
    border: none;
    cursor: pointer;
    transition: color 0.2s;
    font-family: 'Plus Jakarta Sans', sans-serif;
  }
  .nav-row .back-btn { margin-bottom: 0; }
  .back-btn:hover { color: rgb(var(--text-secondary)); }
  .back-btn svg { width: 16px; height: 16px; }
  .switch-mode-btn {
    font-size: 12px;
    color: rgb(var(--text-muted));
    background: rgb(var(--surface-1));
    border: 1px solid rgb(var(--border));
    border-radius: 999px;
    padding: 5px 10px;
    cursor: pointer;
    font-family: 'Plus Jakarta Sans', sans-serif;
  }
  .switch-mode-btn:hover { color: rgb(var(--text-secondary)); background: rgb(var(--surface-2)); }

  .test-result {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    border-radius: 8px;
    font-size: 13px;
  }
  .test-ok { background: rgba(107,144,128,0.08); color: rgb(var(--state-ok)); }
  .test-err { background: rgba(184,92,92,0.08); color: rgb(var(--state-err)); }

  .hint { margin-top: 6px; font-size: 12px; color: rgb(var(--text-muted)); opacity: 0.7; }
  .fingerprint-preview {
    margin-top: 8px; display: flex; flex-wrap: wrap; align-items: center; gap: 6px;
    font-size: 12px; color: rgb(var(--text-muted));
  }
  .fingerprint-preview code { font-family: 'DM Mono', monospace; color: rgb(var(--text-secondary)); }
  .fingerprint-preview em { width: 100%; font-style: normal; font-size: 11px; opacity: 0.75; }
  .fingerprint-preview.ok code { color: rgb(var(--state-ok)); }
  .fingerprint-preview.err { color: rgb(var(--state-err)); }
  .fingerprint-preview.pending { opacity: 0.75; }

  .error-msg {
    font-family: 'DM Mono', monospace;
    font-size: 13px;
    color: rgb(var(--state-err));
    background: rgba(184,92,92,0.06);
    padding: 10px 14px;
    border-radius: 8px;
    border: 1px solid rgba(184,92,92,0.15);
  }

  .info-box {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 10px 14px;
    border-radius: 8px;
    background: rgba(91,127,165,0.06);
    border: 1px solid rgba(91,127,165,0.15);
    color: rgb(var(--state-sync));
    font-size: 13px;
  }

  .step-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: rgb(var(--surface-3));
    transition: all 0.4s cubic-bezier(0.22, 1, 0.36, 1);
  }
  .step-dot.done { background: rgba(196,112,78,0.3); }
  .step-dot.current { background: rgb(var(--accent)); width: 24px; border-radius: 4px; }
</style>
