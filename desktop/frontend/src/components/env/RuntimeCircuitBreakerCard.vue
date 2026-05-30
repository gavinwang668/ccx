<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert } from '@/components/ui/alert'
import { Save, RefreshCw, Zap } from 'lucide-vue-next'
import { useStatus } from '@/composables/useStatus'
import { useLanguage } from '@/composables/useLanguage'
import { GetAdminAccessKey } from '@bindings/github.com/BenedictKing/ccx/desktop/desktopservice'

const { status } = useStatus()
const { t } = useLanguage()

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const success = ref('')
let messageTimer: ReturnType<typeof setTimeout> | null = null

const form = reactive({
  windowSize: 10,
  failureThreshold: 0.5,
  consecutiveFailuresThreshold: 3,
})

const clearMessages = () => {
  error.value = ''
  success.value = ''
  if (messageTimer) {
    clearTimeout(messageTimer)
    messageTimer = null
  }
}

const showMessage = (msg: string, type: 'success' | 'error') => {
  clearMessages()
  if (type === 'success') {
    success.value = msg
  } else {
    error.value = msg
  }
  messageTimer = setTimeout(clearMessages, 5000)
}

const buildApiUrl = async (path: string): Promise<string | null> => {
  if (!status.value.url) return null
  const adminKey = await GetAdminAccessKey()
  if (!adminKey) return null
  return `${status.value.url}${path}`
}

const fetchConfig = async () => {
  const url = await buildApiUrl('/api/settings/circuit-breaker')
  if (!url) return

  loading.value = true
  clearMessages()
  try {
    const adminKey = await GetAdminAccessKey()
    const resp = await fetch(url, {
      headers: { 'x-api-key': adminKey },
    })
    if (!resp.ok) {
      throw new Error(`HTTP ${resp.status}`)
    }
    const data = await resp.json()
    form.windowSize = data.windowSize ?? 10
    form.failureThreshold = data.failureThreshold ?? 0.5
    form.consecutiveFailuresThreshold = data.consecutiveFailuresThreshold ?? 3
  } catch (e) {
    showMessage(t('env.runtimeCbLoadFailed', { error: e instanceof Error ? e.message : String(e) }), 'error')
  } finally {
    loading.value = false
  }
}

const saveConfig = async () => {
  const url = await buildApiUrl('/api/settings/circuit-breaker')
  if (!url) {
    showMessage(t('env.runtimeCbNoBackend'), 'error')
    return
  }

  saving.value = true
  clearMessages()
  try {
    const adminKey = await GetAdminAccessKey()
    const resp = await fetch(url, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'x-api-key': adminKey,
      },
      body: JSON.stringify({
        windowSize: form.windowSize,
        failureThreshold: form.failureThreshold,
        consecutiveFailuresThreshold: form.consecutiveFailuresThreshold,
      }),
    })
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({}))
      throw new Error(body.error || `HTTP ${resp.status}`)
    }
    showMessage(t('env.runtimeCbSaved'), 'success')
  } catch (e) {
    showMessage(t('env.runtimeCbSaveFailed', { error: e instanceof Error ? e.message : String(e) }), 'error')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  if (status.value.running) {
    fetchConfig()
  }
})
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <div class="flex items-start justify-between gap-3">
        <div>
          <CardTitle class="text-base flex items-center gap-2">
            <Zap class="w-4 h-4" />
            {{ t('env.runtimeCbTitle') }}
          </CardTitle>
          <p class="text-xs text-muted-foreground mt-1">
            {{ t('env.runtimeCbDesc') }}
          </p>
        </div>
        <div class="flex gap-2">
          <Button size="sm" variant="ghost" :disabled="loading || !status.running" @click="fetchConfig">
            <RefreshCw class="w-4 h-4 mr-1.5" :class="{ 'animate-spin': loading }" />
            {{ t('env.refresh') }}
          </Button>
          <Button size="sm" :disabled="saving || !status.running" @click="saveConfig">
            <Save class="w-4 h-4 mr-1.5" :class="{ 'animate-spin': saving }" />
            {{ saving ? t('env.saving') : t('env.save') }}
          </Button>
        </div>
      </div>

      <Alert v-if="!status.running" variant="default" class="mt-3">
        <p class="text-sm">{{ t('env.runtimeCbServiceStopped') }}</p>
      </Alert>
      <Alert v-if="error" variant="destructive" class="mt-3">
        <p class="text-sm">{{ error }}</p>
      </Alert>
      <Alert v-if="success" variant="default" class="mt-3">
        <p class="text-sm text-green-600">{{ success }}</p>
      </Alert>
    </CardHeader>

    <CardContent class="space-y-4">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div class="space-y-1.5">
          <Label class="text-xs text-muted-foreground">{{ t('env.runtimeCbWindowSize') }}</Label>
          <Input
            v-model.number="form.windowSize"
            type="number"
            :min="3"
            :max="100"
            :disabled="!status.running"
          />
          <p class="text-xs text-muted-foreground">{{ t('env.runtimeCbWindowSizeDesc') }}</p>
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs text-muted-foreground">{{ t('env.runtimeCbFailureThreshold') }}</Label>
          <Input
            v-model.number="form.failureThreshold"
            type="number"
            :min="0.01"
            :max="1"
            :step="0.01"
            :disabled="!status.running"
          />
          <p class="text-xs text-muted-foreground">{{ t('env.runtimeCbFailureThresholdDesc') }}</p>
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs text-muted-foreground">{{ t('env.runtimeCbConsecutiveFailures') }}</Label>
          <Input
            v-model.number="form.consecutiveFailuresThreshold"
            type="number"
            :min="1"
            :max="100"
            :disabled="!status.running"
          />
          <p class="text-xs text-muted-foreground">{{ t('env.runtimeCbConsecutiveFailuresDesc') }}</p>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
