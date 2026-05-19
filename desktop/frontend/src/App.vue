<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { ApplyAgentConfig, GetAgentConfigStatus, GetStatus, OpenWebUIInBrowser, RestartService, RestoreAgentConfig, ShowStatusTab, ShowWebUITab, StartService, StopService } from '../bindings/github.com/BenedictKing/ccx/desktop/desktopservice'

type HealthInfo = {
  status?: string
  timestamp?: string
  uptime?: number
  mode?: string
  version?: {
    version?: string
    buildTime?: string
    gitCommit?: string
  }
  config?: {
    upstreamCount?: number
  }
}

type DesktopStatus = {
  running: boolean
  starting: boolean
  attached?: boolean
  port: number
  url: string
  pid: number
  binaryPath: string
  dataDir: string
  health?: HealthInfo
  lastError?: string
  logs: string[]
}

type AgentPlatform = 'claude' | 'codex'
type AgentProvider = 'ccx' | 'deepseek' | 'mimo'

type AgentConfigStatus = {
  platform: AgentPlatform
  provider?: string
  targetProvider?: string
  configured: boolean
  matchesCurrentPort: boolean
  needsUpdate: boolean
  currentBaseUrl: string
  targetBaseUrl: string
  configPath: string
  authPath?: string
  hasState: boolean
  lastError?: string
}

type ApplyAgentConfigRequest = {
  platform: AgentPlatform
  provider?: AgentProvider
  apiKey?: string
  baseUrl?: string
}

const activeTab = ref<'status' | 'web'>('status')
const status = ref<DesktopStatus>({
  running: false,
  starting: false,
  attached: false,
  port: 0,
  url: '',
  pid: 0,
  binaryPath: '',
  dataDir: '',
  logs: [],
})
const loading = ref(false)
const configLoading = ref(false)
const actionError = ref('')
const agentStatuses = ref<Record<AgentPlatform, AgentConfigStatus | null>>({
  claude: null,
  codex: null,
})
const selectedClaudeProvider = ref<AgentProvider>('ccx')
const claudeProviderKeys = ref<Record<AgentProvider, string>>({
  ccx: '',
  deepseek: '',
  mimo: '',
})
const claudeMiMoBaseUrl = ref('https://api.mimo.xiaomi.com/v1')

let statusInterval: number | undefined
let unsubscribeTab: (() => void) | undefined
let unsubscribeTrayError: (() => void) | undefined

const syncStatus = async () => {
  try {
    const data = await GetStatus() as DesktopStatus
    status.value = {
      ...status.value,
      ...data,
      logs: Array.isArray(data.logs) ? data.logs : [],
    }
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  }
}

const agentPlatforms: AgentPlatform[] = ['claude', 'codex']

const agentLabels: Record<AgentPlatform, string> = {
  claude: 'Claude Code',
  codex: 'Codex',
}

const claudeProviderLabels: Record<AgentProvider | 'custom', string> = {
  ccx: 'CCX',
  deepseek: 'DeepSeek',
  mimo: 'MiMo',
  custom: '自定义',
}

const isClaudeProvider = (value?: string): value is AgentProvider => {
  return value === 'ccx' || value === 'deepseek' || value === 'mimo'
}

const claudeProviderLabel = (value?: string) => {
  if (!value) return '未识别'
  return claudeProviderLabels[value as AgentProvider | 'custom'] || value
}

const claudeTargetBaseUrl = () => {
  switch (selectedClaudeProvider.value) {
    case 'ccx':
      return agentStatuses.value.claude?.targetBaseUrl || '当前 CCX 网关'
    case 'deepseek':
      return 'https://api.deepseek.com/anthropic'
    case 'mimo':
      return claudeMiMoBaseUrl.value || 'https://api.mimo.xiaomi.com/v1'
  }
}

const canApplyAgent = (platform: AgentPlatform) => {
  if (configLoading.value) return false
  if (platform === 'codex') return status.value.running
  if (selectedClaudeProvider.value === 'ccx') return status.value.running
  return claudeProviderKeys.value[selectedClaudeProvider.value].trim() !== ''
}

const loadAgentStatuses = async () => {
  configLoading.value = true
  try {
    const [claude, codex] = await Promise.all([
      GetAgentConfigStatus('claude') as Promise<AgentConfigStatus>,
      GetAgentConfigStatus('codex') as Promise<AgentConfigStatus>,
    ])
    agentStatuses.value = { claude, codex }
    if (isClaudeProvider(claude.provider)) {
      selectedClaudeProvider.value = claude.provider
    }
    if (claude.provider === 'mimo' && claude.currentBaseUrl) {
      claudeMiMoBaseUrl.value = claude.currentBaseUrl
    }
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  } finally {
    configLoading.value = false
  }
}

const applyAgent = async (platform: AgentPlatform) => {
  actionError.value = ''
  configLoading.value = true
  try {
    const request: ApplyAgentConfigRequest = { platform }
    if (platform === 'claude') {
      request.provider = selectedClaudeProvider.value
      if (selectedClaudeProvider.value !== 'ccx') {
        request.apiKey = claudeProviderKeys.value[selectedClaudeProvider.value].trim()
      }
      if (selectedClaudeProvider.value === 'mimo') {
        request.baseUrl = claudeMiMoBaseUrl.value.trim()
      }
    }
    await ApplyAgentConfig(request)
    if (platform === 'claude' && selectedClaudeProvider.value !== 'ccx') {
      claudeProviderKeys.value[selectedClaudeProvider.value] = ''
    }
    await loadAgentStatuses()
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  } finally {
    configLoading.value = false
  }
}

const restoreAgent = async (platform: AgentPlatform) => {
  actionError.value = ''
  configLoading.value = true
  try {
    await RestoreAgentConfig(platform)
    await loadAgentStatuses()
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  } finally {
    configLoading.value = false
  }
}

const agentStatusText = (item: AgentConfigStatus | null) => {
  if (!item) return '检测中'
  if (item.configured) return '已配置'
  if (item.needsUpdate) return '端口不匹配'
  return '未配置'
}

const agentStatusClass = (item: AgentConfigStatus | null) => {
  if (!item) return 'starting'
  if (item.configured) return 'running'
  if (item.needsUpdate) return 'starting'
  return 'stopped'
}

const refresh = async () => {
  loading.value = true
  try {
    await syncStatus()
  } finally {
    loading.value = false
  }
}

const invoke = async (action: () => Promise<unknown>) => {
  actionError.value = ''
  try {
    await action()
    await syncStatus()
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  }
}

const startService = () => invoke(StartService)
const stopService = () => invoke(StopService)
const restartService = () => invoke(RestartService)
const openInBrowser = () => invoke(OpenWebUIInBrowser)
const showWebTab = async () => {
  actionError.value = ''
  try {
    await ShowWebUITab()
    activeTab.value = 'web'
    await syncStatus()
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  }
}
const showStatusTab = async () => {
  actionError.value = ''
  try {
    await ShowStatusTab()
    activeTab.value = 'status'
    await syncStatus()
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  }
}

onMounted(async () => {
  await syncStatus()
  statusInterval = window.setInterval(syncStatus, 3000)
  unsubscribeTab = Events.On('desktop:show-tab', (event: { data: string }) => {
    if (event.data === 'status' || event.data === 'web') {
      activeTab.value = event.data
    }
  })
  unsubscribeTrayError = Events.On('desktop:tray-error', (event: { data: string }) => {
    actionError.value = event.data
    void syncStatus()
  })
})

onBeforeUnmount(() => {
  if (statusInterval) {
    window.clearInterval(statusInterval)
  }
  unsubscribeTab?.()
  unsubscribeTrayError?.()
})
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <div>
        <p class="eyebrow">CCX Desktop</p>
        <h1>CCX 桌面外壳</h1>
      </div>
      <div class="status-pill" :class="status.running ? 'running' : status.starting ? 'starting' : 'stopped'">
        {{ status.running ? '运行中' : status.starting ? '启动中' : '已停止' }}
      </div>
    </header>

    <nav class="tabs">
      <button :class="['tab', activeTab === 'status' ? 'active' : '']" @click="activeTab = 'status'">状态</button>
      <button :class="['tab', activeTab === 'web' ? 'active' : '']" @click="showWebTab">
        CCX Web UI
      </button>
    </nav>

    <section v-if="activeTab === 'status'" class="panel">
      <div class="metrics">
        <article><span>端口</span><strong>{{ status.port }}</strong></article>
        <article><span>版本</span><strong>{{ status.health?.version?.version || 'v0.0.0-dev' }}</strong></article>
        <article><span>运行时长</span><strong>{{ status.health?.uptime ? `${Math.floor(status.health.uptime / 60)}m` : '--' }}</strong></article>
        <article><span>上游数</span><strong>{{ status.health?.config?.upstreamCount || 0 }}</strong></article>
      </div>

      <div class="actions">
        <button @click="startService" :disabled="loading || status.running">启动</button>
        <button @click="stopService" :disabled="loading || !status.running || status.attached">停止</button>
        <button @click="restartService" :disabled="loading || status.attached">重启</button>
        <button @click="showWebTab" :disabled="loading">打开 Web UI</button>
        <button @click="openInBrowser" :disabled="loading">浏览器打开</button>
        <button @click="refresh" :disabled="loading">刷新</button>
      </div>

      <p v-if="actionError" class="error">{{ actionError }}</p>
      <p v-else-if="status.lastError" class="error">{{ status.lastError }}</p>

      <div class="details">
        <div><span>二进制</span><code>{{ status.binaryPath || '未发现' }}</code></div>
        <div><span>数据目录</span><code>{{ status.dataDir || '未设置' }}</code></div>
        <div><span>PID</span><code>{{ status.pid || '-' }}</code></div>
        <div><span>健康状态</span><code>{{ status.health?.status || 'unknown' }}</code></div>
      </div>

      <div class="agent-section">
        <header class="web-header">
          <div>
            <p class="eyebrow">Agent 配置</p>
            <h2>Claude Code / Codex</h2>
          </div>
          <button @click="loadAgentStatuses" :disabled="configLoading">刷新状态</button>
        </header>

        <p class="hint">
          Claude Code 可切换到 CCX、DeepSeek 或 MiMo；Codex 固定访问当前 CCX 网关。恢复只回滚 Desktop 写入的字段。
        </p>

        <div class="agent-grid">
          <article v-for="platform in agentPlatforms" :key="platform" class="agent-card">
            <header class="agent-card-header">
              <div>
                <p class="eyebrow">{{ platform }}</p>
                <h3>{{ agentLabels[platform] }}</h3>
              </div>
              <span :class="['status-pill', agentStatusClass(agentStatuses[platform])]">
                {{ agentStatusText(agentStatuses[platform]) }}
              </span>
            </header>

            <div class="details compact">
              <div><span>当前 Provider</span><code>{{ claudeProviderLabel(agentStatuses[platform]?.provider) }}</code></div>
              <div><span>当前 URL</span><code>{{ agentStatuses[platform]?.currentBaseUrl || '未设置' }}</code></div>
              <div><span>目标 URL</span><code>{{ platform === 'claude' ? claudeTargetBaseUrl() : agentStatuses[platform]?.targetBaseUrl || '--' }}</code></div>
              <div><span>配置文件</span><code>{{ agentStatuses[platform]?.configPath || '--' }}</code></div>
              <div v-if="agentStatuses[platform]?.authPath"><span>认证文件</span><code>{{ agentStatuses[platform]?.authPath }}</code></div>
            </div>

            <div v-if="platform === 'claude'" class="provider-form">
              <label>
                Provider
                <select v-model="selectedClaudeProvider">
                  <option value="ccx">CCX 本地网关</option>
                  <option value="deepseek">DeepSeek 直连</option>
                  <option value="mimo">MiMo 直连</option>
                </select>
              </label>
              <label v-if="selectedClaudeProvider !== 'ccx'">
                API Key
                <input v-model="claudeProviderKeys[selectedClaudeProvider]" type="password" autocomplete="off" placeholder="仅写入 Claude Code 配置" />
              </label>
              <label v-if="selectedClaudeProvider === 'mimo'">
                MiMo Base URL
                <input v-model="claudeMiMoBaseUrl" type="url" placeholder="https://api.mimo.xiaomi.com/v1" />
              </label>
            </div>
            <p v-else class="hint compact-hint">Codex 固定写入 CCX Responses 配置，暂不直连厂商。</p>

            <p v-if="agentStatuses[platform]?.lastError" class="error">{{ agentStatuses[platform]?.lastError }}</p>

            <div class="actions">
              <button @click="applyAgent(platform)" :disabled="!canApplyAgent(platform)">
                {{ platform === 'claude' ? `应用 ${claudeProviderLabel(selectedClaudeProvider)} 配置` : '应用 CCX 配置' }}
              </button>
              <button @click="restoreAgent(platform)" :disabled="configLoading || !agentStatuses[platform]?.hasState">
                恢复原始配置
              </button>
            </div>
          </article>
        </div>
      </div>

      <div class="logs">
        <header>
          <h2>最近日志</h2>
        </header>
        <pre>{{ status.logs.length ? status.logs.join('\n') : '暂无日志' }}</pre>
      </div>
    </section>

    <section v-else-if="activeTab === 'web'" class="web-panel panel">
      <header class="web-header">
        <div>
          <p class="eyebrow">内置标签页</p>
          <h2>CCX Web UI</h2>
        </div>
        <div class="web-actions">
          <button @click="showStatusTab">返回状态页</button>
          <button @click="openInBrowser" :disabled="loading">浏览器打开</button>
        </div>
      </header>
      <div v-if="status.running && status.url" class="web-frame-wrap">
        <iframe :src="status.url" class="web-frame" title="CCX Web UI"></iframe>
      </div>
      <div v-else class="placeholder">
        <p>CCX 服务尚未启动，无法显示 Web UI。</p>
        <button @click="showWebTab" :disabled="loading">立即启动</button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  padding: 0;
  color: #e5eefc;
  background: radial-gradient(circle at top, #21314f 0, #0d1422 48%, #080d17 100%);
}

.topbar,
.web-header,
.metrics,
.actions,
.details,
.tabs,
.agent-card-header {
  display: flex;
  gap: 12px;
}

.topbar,
.web-header,
.agent-card-header {
  align-items: center;
  justify-content: space-between;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: #88a0c8;
}

h1,
h2,
h3,
p {
  margin: 0;
}

.status-pill {
  padding: 8px 14px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 700;
}

.status-pill.running {
  background: rgba(70, 180, 120, 0.2);
  color: #7dffb8;
}

.status-pill.starting {
  background: rgba(255, 196, 87, 0.2);
  color: #ffd26b;
}

.status-pill.stopped {
  background: rgba(255, 107, 107, 0.16);
  color: #ff9b9b;
}

.topbar,
.tabs {
  padding: 20px 24px 0;
}

.tabs {
  margin: 0 0 20px;
}

.tab,
.actions button,
.web-actions button,
.placeholder button,
.web-header button {
  border: 0;
  border-radius: 10px;
  padding: 10px 14px;
  background: #1b2640;
  color: #dfe8fb;
  cursor: pointer;
}

.tab.active {
  background: #4d6bff;
  color: white;
}

.tab:disabled,
.actions button:disabled,
.web-actions button:disabled,
.placeholder button:disabled,
.web-header button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.panel {
  display: grid;
  gap: 16px;
  padding: 20px 24px 24px;
  border: 0;
  border-radius: 0;
  background: transparent;
  backdrop-filter: none;
}

.metrics {
  flex-wrap: wrap;
}

.metrics article,
.details div,
.agent-card {
  flex: 1 1 180px;
  padding: 14px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.04);
}

.metrics span,
.details span {
  display: block;
  font-size: 12px;
  color: #90a2c9;
  margin-bottom: 8px;
}

.metrics strong,
.details code {
  font-size: 18px;
  word-break: break-word;
}

.actions {
  flex-wrap: wrap;
}

.error {
  color: #ff9b9b;
}

.logs pre {
  margin: 0;
  max-height: 260px;
  overflow: auto;
  padding: 16px;
  border-radius: 14px;
  background: #04070d;
  color: #c7d3e8;
  white-space: pre-wrap;
}

.web-frame-wrap {
  min-height: 620px;
  border-radius: 16px;
  overflow: hidden;
  border: 1px solid rgba(137, 163, 214, 0.16);
}

.web-frame {
  width: 100%;
  height: 100%;
  min-height: 620px;
  border: 0;
  background: white;
}

.placeholder {
  display: grid;
  gap: 12px;
  justify-items: start;
}

.hint {
  color: #a8b7d6;
}

.agent-section {
  display: grid;
  gap: 14px;
  padding: 14px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.04);
}

.agent-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.agent-card {
  display: grid;
  gap: 14px;
  align-items: stretch;
}

.details.compact {
  display: grid;
}

.details.compact div {
  flex: unset;
}

.provider-form {
  display: grid;
  gap: 10px;
}

.provider-form label {
  display: grid;
  gap: 6px;
  color: #90a2c9;
  font-size: 12px;
}

.provider-form select,
.provider-form input {
  min-width: 0;
  border: 1px solid rgba(137, 163, 214, 0.2);
  border-radius: 10px;
  padding: 10px 12px;
  background: #0d1422;
  color: #dfe8fb;
}

</style>

<style>
body {
  margin: 0;
}
</style>
