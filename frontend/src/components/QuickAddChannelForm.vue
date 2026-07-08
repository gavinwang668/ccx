<template>
  <div class="quick-add-form d-flex flex-column ga-4">
    <!-- 服务类型选择 -->
    <div class="d-flex align-center ga-2">
      <v-icon :color="serviceTypeColor" size="20">{{ serviceTypeIcon }}</v-icon>
      <div class="text-caption text-medium-emphasis flex-shrink-0">
        {{ t('channelEditor.basic.serviceType.label') }}
      </div>
      <v-spacer />
      <v-select
        v-model="serviceType"
        :items="serviceTypeItems"
        item-title="title"
        item-value="value"
        variant="outlined"
        density="compact"
        hide-details
        :menu-props="{ contentClass: 'upstream-select-menu' }"
        class="service-type-select"
        @update:model-value="onServiceTypeChange"
      />
    </div>

    <!-- 名称（可选） -->
    <v-text-field
      v-model="channelName"
      :label="t('addChannel.channelName')"
      :placeholder="t('autopilot.quickAdd.namePlaceholder')"
      variant="outlined"
      density="compact"
      hide-details
      prepend-inner-icon="mdi-tag"
    />

    <!-- Base URL 输入 -->
    <div>
      <div class="d-flex align-center justify-space-between mb-2">
        <div class="d-flex align-center ga-2">
          <v-icon size="16" color="medium-emphasis">mdi-web</v-icon>
          <span class="text-body-2 font-weight-medium">{{ t('addChannel.baseUrl') }}</span>
        </div>
        <v-btn size="small" variant="text" color="primary" @click="addBaseUrl">
          <v-icon start size="small">mdi-plus</v-icon>
          {{ t('autopilot.quickAdd.addUrl') }}
        </v-btn>
      </div>
      <div class="d-flex flex-column ga-2">
        <div
          v-for="(url, idx) in baseUrls"
          :key="'url-' + idx"
          class="d-flex align-center ga-2"
        >
          <v-text-field
            v-model="baseUrls[idx]"
            :placeholder="t('addChannel.baseUrl') + ' ' + (idx + 1)"
            variant="outlined"
            density="compact"
            hide-details
            class="flex-grow-1"
            @input="validateForm"
          />
          <v-btn
            v-if="baseUrls.length > 1"
            size="small"
            icon
            variant="text"
            color="error"
            @click="removeBaseUrl(idx)"
          >
            <v-icon size="small">mdi-close</v-icon>
          </v-btn>
        </div>
      </div>
    </div>

    <!-- API Key 输入（Copilot 模式下可选） -->
    <div>
      <div class="d-flex align-center justify-space-between mb-2">
        <div class="d-flex align-center ga-2">
          <v-icon size="16" color="medium-emphasis">mdi-key</v-icon>
          <span class="text-body-2 font-weight-medium">{{ t('addChannel.apiKeys') }}</span>
          <v-chip
            v-if="isCopilot"
            size="x-small"
            color="info"
            variant="tonal"
          >
            {{ t('autopilot.quickAdd.optionalForCopilot') }}
          </v-chip>
        </div>
        <v-btn size="small" variant="text" color="primary" @click="addApiKey">
          <v-icon start size="small">mdi-plus</v-icon>
          {{ t('autopilot.quickAdd.addKey') }}
        </v-btn>
      </div>
      <div class="d-flex flex-column ga-2">
        <div
          v-for="(key, idx) in apiKeys"
          :key="'key-' + idx"
          class="d-flex align-center ga-2"
        >
          <v-text-field
            v-model="apiKeys[idx]"
            :placeholder="'sk-...' + (idx + 1)"
            variant="outlined"
            density="compact"
            hide-details
            :type="showKeys[idx] ? 'text' : 'password'"
            class="flex-grow-1"
            @input="validateForm"
          >
            <template #append-inner>
              <v-icon
                size="small"
                class="cursor-pointer"
                @click="toggleKeyVisibility(idx)"
              >
                {{ showKeys[idx] ? 'mdi-eye-off' : 'mdi-eye' }}
              </v-icon>
            </template>
          </v-text-field>
          <v-btn
            v-if="apiKeys.length > 1"
            size="small"
            icon
            variant="text"
            color="error"
            @click="removeApiKey(idx)"
          >
            <v-icon size="small">mdi-close</v-icon>
          </v-btn>
        </div>
      </div>
    </div>

    <!-- Copilot OAuth 提示 -->
    <v-alert
      v-if="isCopilot"
      color="info"
      variant="tonal"
      density="comfortable"
    >
      {{ t('copilotOAuth.quickAddHint') }}
    </v-alert>

    <!-- 自动托管开关 -->
    <v-card variant="outlined" class="auto-managed-card" rounded="lg">
      <v-card-text class="pa-3">
        <div class="d-flex align-center ga-3">
          <v-checkbox
            v-model="autoManaged"
            color="primary"
            density="compact"
            hide-details
            class="ma-0 pa-0 flex-shrink-0"
          />
          <div class="flex-grow-1">
            <div class="text-body-2 font-weight-medium">
              {{ t('autopilot.quickAdd.autoManaged') }}
            </div>
            <div class="text-caption text-medium-emphasis">
              {{ t('autopilot.quickAdd.autoManagedHint') }}
            </div>
          </div>
          <v-icon color="primary" size="24">mdi-auto-fix</v-icon>
        </div>
      </v-card-text>
    </v-card>

    <!-- 发现状态面板 -->
    <v-card v-if="submitting" variant="outlined" class="discovery-card" rounded="lg">
      <v-card-text class="pa-4">
        <div class="d-flex align-center ga-3 mb-3">
          <v-progress-circular
            v-if="autoStatus.status === 'discovering'"
            indeterminate
            size="20"
            width="2"
            color="primary"
          />
          <v-icon v-else-if="autoStatus.status === 'done'" color="success" size="20">mdi-check-circle</v-icon>
          <v-icon v-else-if="autoStatus.status === 'failed'" color="error" size="20">mdi-alert-circle</v-icon>
          <span class="text-body-2 font-weight-medium">{{ statusText }}</span>
        </div>

        <template v-if="autoStatus.endpoints.length > 0">
          <v-divider class="mb-3" />
          <div class="d-flex flex-column ga-2">
            <div
              v-for="(ep, idx) in autoStatus.endpoints"
              :key="idx"
              class="d-flex align-center ga-2 text-caption"
            >
              <v-icon size="14" :color="ep.protocolOk ? 'success' : 'error'">
                {{ ep.protocolOk ? 'mdi-check-circle' : 'mdi-close-circle' }}
              </v-icon>
              <code class="text-caption">{{ ep.keyMask }}</code>
              <v-spacer />
              <span v-if="ep.modelsCount > 0" class="text-success">
                {{ ep.modelsCount }} {{ t('autopilot.quickAdd.models') }}
              </span>
            </div>
          </div>
        </template>
      </v-card-text>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from '../i18n'
import { autoAddChannel, getChannelAutoStatus } from '../services/autopilot-api'

type ServiceType = 'openai' | 'gemini' | 'claude' | 'responses' | 'copilot'
type ChannelType = 'messages' | 'chat' | 'responses' | 'gemini' | 'images' | 'vectors'

interface Props {
  channelType: ChannelType
}

const props = defineProps<Props>()

const emit = defineEmits<{
  added: [channelId: number]
  close: []
}>()

const { t } = useI18n()

// ---- 表单状态 ----
const serviceType = ref<ServiceType>(getDefaultServiceType())
const channelName = ref('')
const baseUrls = ref<string[]>([''])
const apiKeys = ref<string[]>([''])
const showKeys = ref<boolean[]>([false])
const autoManaged = ref(true)
const submitting = ref(false)

// ---- 发现状态 ----
import type { AutoEndpointStatus } from '../services/autopilot-api'

const autoStatus = reactive({
  status: '' as 'discovering' | 'done' | 'failed' | '',
  endpoints: [] as AutoEndpointStatus[],
})

let pollTimer: ReturnType<typeof setInterval> | null = null

// ---- 常量 ----
const serviceTypeItems = computed(() => {
  if (props.channelType === 'images' || props.channelType === 'vectors') {
    return [{ title: 'OpenAI Images', value: 'openai' as const }]
  }
  return [
    { title: 'Claude', value: 'claude' as const },
    { title: 'OpenAI Chat', value: 'openai' as const },
    { title: 'Gemini', value: 'gemini' as const },
    { title: 'Responses (Codex)', value: 'responses' as const },
    { title: 'GitHub Copilot', value: 'copilot' as const },
  ]
})

const serviceTypeIcon = computed(() => {
  const map: Record<ServiceType, string> = {
    openai: 'mdi-robot',
    claude: 'mdi-message-processing',
    gemini: 'mdi-diamond-stone',
    responses: 'mdi-rocket-launch',
    copilot: 'mdi-code-braces',
  }
  return map[serviceType.value] || 'mdi-api'
})

const serviceTypeColor = computed(() => {
  const map: Record<ServiceType, string> = {
    openai: 'info',
    claude: 'orange',
    gemini: 'purple',
    responses: 'success',
    copilot: 'grey-darken-2',
  }
  return map[serviceType.value] || 'primary'
})

const isCopilot = computed(() => serviceType.value === 'copilot')

const isFormValid = computed(() => {
  const hasUrl = baseUrls.value.some(u => u.trim() !== '')
  if (isCopilot.value) return hasUrl
  return hasUrl && apiKeys.value.some(k => k.trim() !== '')
})

const statusText = computed(() => {
  switch (autoStatus.status) {
    case 'discovering': return t('autopilot.quickAdd.discovering')
    case 'done': return t('autopilot.quickAdd.discoveryDone')
    case 'failed': return t('autopilot.quickAdd.discoveryFailed')
    default: return ''
  }
})

// ---- 方法 ----
function getDefaultServiceType(): ServiceType {
  if (props.channelType === 'chat') return 'openai'
  if (props.channelType === 'gemini') return 'gemini'
  if (props.channelType === 'responses') return 'responses'
  if (props.channelType === 'images' || props.channelType === 'vectors') return 'openai'
  return 'claude'
}

function onServiceTypeChange() {
  if (isCopilot.value && baseUrls.value.length === 1 && !baseUrls.value[0]) {
    baseUrls.value = ['https://api.githubcopilot.com']
  }
}

function addBaseUrl() {
  baseUrls.value.push('')
}

function removeBaseUrl(idx: number) {
  baseUrls.value.splice(idx, 1)
}

function addApiKey() {
  apiKeys.value.push('')
  showKeys.value.push(false)
}

function removeApiKey(idx: number) {
  apiKeys.value.splice(idx, 1)
  showKeys.value.splice(idx, 1)
}

function toggleKeyVisibility(idx: number) {
  showKeys.value[idx] = !showKeys.value[idx]
}

function validateForm() {
  // 触发响应式更新
}

function getFilteredBaseUrls(): string[] {
  return baseUrls.value.filter(u => u.trim() !== '')
}

function getFilteredApiKeys(): string[] {
  return apiKeys.value.filter(k => k.trim() !== '')
}

function generateRandomSuffix(length = 6): string {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let result = ''
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return result
}

function getGeneratedName(): string {
  const filtered = getFilteredBaseUrls()
  const first = filtered[0] || ''
  try {
    const host = new URL(first).hostname.replace(/\./g, '-')
    return `${host}-${generateRandomSuffix()}`
  } catch {
    return `channel-${generateRandomSuffix()}`
  }
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function startPolling(kind: ChannelType, channelId: number) {
  let attempts = 0
  const maxAttempts = 60 // 最多 5 分钟 (5s * 60)

  pollTimer = setInterval(async () => {
    attempts++
    if (attempts > maxAttempts) {
      stopPolling()
      autoStatus.status = 'failed'
      return
    }

    try {
      const result = await getChannelAutoStatus(kind, channelId)
      const discovery = result.discovery
      if (!discovery) return // 尚未触发发现
      if (discovery.status === 'done') {
        stopPolling()
        autoStatus.status = 'done'
        autoStatus.endpoints = discovery.endpoints || []
      } else if (discovery.status === 'failed') {
        stopPolling()
        autoStatus.status = 'failed'
        autoStatus.endpoints = discovery.endpoints || []
      } else {
        autoStatus.endpoints = discovery.endpoints || []
      }
    } catch {
      // 忽略轮询错误，继续尝试
    }
  }, 5000)
}

async function handleSubmit() {
  if (!isFormValid.value || submitting.value) return

  submitting.value = true
  autoStatus.status = 'discovering'
  autoStatus.endpoints = []

  const finalName = channelName.value.trim() || getGeneratedName()

  try {
    const result = await autoAddChannel(props.channelType, {
      name: finalName,
      baseUrls: getFilteredBaseUrls(),
      apiKeys: getFilteredApiKeys(),
    })

    if (result.discoveryStarted) {
      startPolling(props.channelType, result.index)
    }

    emit('added', result.index)
  } catch (err) {
    stopPolling()
    submitting.value = false
    autoStatus.status = 'failed'
    console.error('[QuickAdd-Submit] 自动添加渠道失败:', err)
  }
}

function resetForm() {
  serviceType.value = getDefaultServiceType()
  channelName.value = ''
  baseUrls.value = ['']
  apiKeys.value = ['']
  showKeys.value = [false]
  autoManaged.value = true
  submitting.value = false
  autoStatus.status = ''
  autoStatus.endpoints = []
  stopPolling()
}

// ---- 生命周期 ----
onUnmounted(() => {
  stopPolling()
})

// 暴露给父组件
defineExpose({ handleSubmit, resetForm, isFormValid, submitting })
</script>

<style scoped>
.quick-add-form {
  min-height: 0;
}

.service-type-select {
  min-width: 200px;
  max-width: 260px;
}

.auto-managed-card {
  border-color: rgba(var(--v-theme-primary), 0.3);
  background: rgba(var(--v-theme-primary), 0.03);
}

.discovery-card {
  border-color: rgba(var(--v-theme-outline), 0.32);
}
</style>
