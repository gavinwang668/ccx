<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Check, CheckCircle2, Copy, Github, Loader2, ShieldCheck, Trash2, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAdminApi } from '@/composables/useAdminApi'
import { useChannelPresets } from '@/composables/useChannelPresets'
import { useCopilotOAuth } from '@/composables/useCopilotOAuth'
import { buildBaseConfigs, maskKey, useCopilotAccounts } from '@/composables/useCopilotAccounts'
import { useLanguage } from '@/composables/useLanguage'
import { GetProviderKeyAssets } from '@bindings/github.com/BenedictKing/ccx/desktop/desktopservice'
import type { Channel, ChannelsResponse } from '@/services/admin-api'
import type { ProviderKeyAsset } from '@/types'

type CopilotTarget = 'messages' | 'chat' | 'responses' | 'gemini'

const { t } = useLanguage()
const adminApi = useAdminApi()
const { creating, error, createChannel } = useChannelPresets()
const { verifyAccount, addAccount, removeAccount } = useCopilotAccounts()

const copilotApiKeys = ref<string[]>([])
const copilotProxyUrl = ref('')
const selectedCopilotTarget = ref<CopilotTarget>('responses')
const copilotCreateError = ref('')
const accountActionError = ref('')
const existingCopilotChannels = ref<Record<CopilotTarget, Channel | null>>({
  messages: null,
  chat: null,
  responses: null,
  gemini: null,
})
const savedCopilotAsset = ref<ProviderKeyAsset | null>(null)
const checkingCopilotChannel = ref(false)
const addingCopilotChannel = ref(false)
const verifyingAccount = ref(false)
const removingKey = ref('')
const pendingRemoveKey = ref('')

const {
  copilotOAuthLoading,
  copilotPolling,
  copilotOAuthError,
  copilotOAuthSuccess,
  copilotUserCode,
  copilotUserCodeCopied,
  clearCopilotPollTimer,
  copyCopilotUserCode,
  startCopilotOAuth,
  openCopilotAuthorization,
} = useCopilotOAuth(copilotApiKeys, t, () => copilotProxyUrl.value)

const latestAuthorizedCopilotToken = computed(() => copilotApiKeys.value[copilotApiKeys.value.length - 1] || '')
const savedCopilotToken = computed(() => savedCopilotAsset.value?.apiKey || '')
const availableCopilotToken = computed(() => latestAuthorizedCopilotToken.value || savedCopilotToken.value)
const selectedCopilotChannel = computed(() => existingCopilotChannels.value[selectedCopilotTarget.value])
const hasCopilotChannel = computed(() => Boolean(selectedCopilotChannel.value))
const hasSavedCopilotAuthorization = computed(() => Boolean(savedCopilotToken.value))
const copilotAccounts = computed(() => (selectedCopilotChannel.value ? buildBaseConfigs(selectedCopilotChannel.value) : []))
const accountCount = computed(() => copilotAccounts.value.length)
const copilotBusy = computed(() =>
  copilotOAuthLoading.value || copilotPolling.value || creating.value || addingCopilotChannel.value || verifyingAccount.value,
)
const copilotTargetOptions = computed<Array<{ value: CopilotTarget; label: string; description: string }>>(() => [
  { value: 'messages', label: t('subscription.targetClaude'), description: t('subscription.targetClaudeDesc') },
  { value: 'chat', label: t('subscription.targetChat'), description: t('subscription.targetChatDesc') },
  { value: 'responses', label: t('subscription.targetCodex'), description: t('subscription.targetCodexDesc') },
  { value: 'gemini', label: t('subscription.targetGemini'), description: t('subscription.targetGeminiDesc') },
])
const selectedCopilotTargetOption = computed(() =>
  copilotTargetOptions.value.find(item => item.value === selectedCopilotTarget.value) || copilotTargetOptions.value[2],
)
const copilotPrimaryActionLabel = computed(() =>
  hasCopilotChannel.value ? t('subscription.authorizeAndAddAccount') : t('subscription.authorizeCopilot'),
)

async function refreshSavedCopilotAsset() {
  try {
    const assets = await GetProviderKeyAssets() as ProviderKeyAsset[]
    const asset = assets.find(item => item.provider === 'github-copilot' && item.apiKey) || null
    savedCopilotAsset.value = asset
    if (asset?.proxyUrl && !copilotProxyUrl.value.trim()) {
      copilotProxyUrl.value = asset.proxyUrl
    }
  } catch {
    savedCopilotAsset.value = null
  }
}

async function refreshCopilotChannelStatus() {
  checkingCopilotChannel.value = true
  try {
    const entries = await Promise.all(
      copilotTargetOptions.value.map(async ({ value }) => {
        const data = await adminApi.get<ChannelsResponse>(`/api/${value}/channels`)
        const channel = data.channels.find(item => item.name === 'desktop-github-copilot')
          || data.channels.find(item => item.serviceType === 'copilot')
          || null
        return [value, channel] as const
      }),
    )
    existingCopilotChannels.value = Object.fromEntries(entries) as Record<CopilotTarget, Channel | null>
    const channelWithProxy = entries.map(([, channel]) => channel).find(channel => channel?.proxyUrl)
    if (channelWithProxy?.proxyUrl && !copilotProxyUrl.value.trim()) {
      copilotProxyUrl.value = channelWithProxy.proxyUrl
    }
  } catch {
    existingCopilotChannels.value = {
      messages: null,
      chat: null,
      responses: null,
      gemini: null,
    }
  } finally {
    checkingCopilotChannel.value = false
  }
}

async function startCopilotAuthorization() {
  copilotApiKeys.value = []
  copilotCreateError.value = ''
  accountActionError.value = ''
  await startCopilotOAuth()
}

function handleCopilotPrimaryAction() {
  void startCopilotAuthorization()
}

function cancelCopilotAuthorization() {
  clearCopilotPollTimer()
  copilotPolling.value = false
  copilotOAuthLoading.value = false
}

// 授权拿到新 token 后：反查 GitHub 用户名 -> 必要时建渠道 -> 合并进渠道 key 池。
async function processNewToken(token: string) {
  if (!token || verifyingAccount.value || addingCopilotChannel.value) return
  const target = selectedCopilotTarget.value
  accountActionError.value = ''
  copilotCreateError.value = ''
  verifyingAccount.value = true
  try {
    const login = await verifyAccount(token, copilotProxyUrl.value)
    if (!login) throw new Error(t('subscription.verifyFailed'))
    verifyingAccount.value = false
    addingCopilotChannel.value = true
    let channel = existingCopilotChannels.value[target]
    if (!channel) {
      await createChannel({
        provider: 'github-copilot',
        target,
        baseUrl: 'https://api.githubcopilot.com',
        apiKey: token,
        name: 'desktop-github-copilot',
        proxyUrl: copilotProxyUrl.value.trim(),
      }, { reloadPresets: false })
      await refreshCopilotChannelStatus()
      channel = existingCopilotChannels.value[target]
    }
    if (!channel) throw new Error(t('subscription.channelResolveFailed'))
    await addAccount(target, channel, token, login)
    await refreshSavedCopilotAsset()
    await refreshCopilotChannelStatus()
  } catch (err) {
    accountActionError.value = err instanceof Error ? err.message : String(err)
  } finally {
    verifyingAccount.value = false
    addingCopilotChannel.value = false
  }
}

// 加入失败后用已授权 token 重试，避免重新走 OAuth。
function retryAddAccount() {
  const token = latestAuthorizedCopilotToken.value
  if (token && !copilotBusy.value) void processNewToken(token)
}

async function removeAccountConfirmed(key: string) {
  const target = selectedCopilotTarget.value
  const channel = existingCopilotChannels.value[target]
  if (!channel || removingKey.value) return
  accountActionError.value = ''
  pendingRemoveKey.value = ''
  removingKey.value = key
  try {
    await removeAccount(target, channel, key)
    await refreshCopilotChannelStatus()
  } catch (err) {
    accountActionError.value = err instanceof Error ? err.message : String(err)
  } finally {
    removingKey.value = ''
  }
}

watch(latestAuthorizedCopilotToken, (token) => {
  if (token) void processNewToken(token)
})

onMounted(() => {
  void refreshSavedCopilotAsset()
  void refreshCopilotChannelStatus()
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col gap-5">
    <div class="bg-glass dark:bg-glass-dark border border-border rounded-2xl p-5 shrink-0">
      <div class="flex items-start justify-between gap-4">
        <div>
          <div class="flex items-center gap-2 text-primary mb-2">
            <ShieldCheck class="w-4 h-4" />
            <span class="text-xs font-bold uppercase tracking-[0.2em]">{{ t('subscription.headerEyebrow') }}</span>
          </div>
          <h3 class="text-xl font-bold text-foreground">{{ t('subscription.title') }}</h3>
          <p class="text-sm text-muted-foreground mt-1 max-w-2xl">
            {{ t('subscription.description') }}
          </p>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,520px)_1fr]">
      <section class="bg-glass dark:bg-glass-dark border border-border rounded-2xl p-5 space-y-5">
        <div class="flex items-start gap-3">
          <div class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-secondary ring-1 ring-border">
            <Github class="h-5 w-5 text-foreground" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <h4 class="text-base font-semibold text-foreground">GitHub Copilot</h4>
              <span
                v-if="copilotOAuthSuccess || hasSavedCopilotAuthorization || accountCount > 0"
                class="rounded border border-emerald-500/20 bg-emerald-500/10 px-1.5 py-0.5 text-[10px] text-emerald-700 dark:text-emerald-400"
              >
                {{ t('subscription.authorized') }}
              </span>
            </div>
            <p class="mt-1 text-sm text-muted-foreground">{{ t('subscription.copilotDescription') }}</p>
          </div>
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs text-muted-foreground">{{ t('channelEditor.transport.proxyUrl.label') }}</Label>
          <Input
            v-model="copilotProxyUrl"
            class="font-mono text-xs"
            :placeholder="t('channelEditor.transport.proxyUrl.placeholder')"
          />
          <p class="text-xs text-muted-foreground">{{ t('channelEditor.transport.proxyUrl.hint') }}</p>
        </div>

        <div class="space-y-2">
          <Label class="text-xs text-muted-foreground">{{ t('subscription.targetLabel') }}</Label>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <button
              v-for="target in copilotTargetOptions"
              :key="target.value"
              type="button"
              :class="[
                'rounded-lg border px-3 py-2 text-left transition-colors',
                selectedCopilotTarget === target.value
                  ? 'border-primary/50 bg-primary/10 text-primary'
                  : 'border-border bg-background/70 text-foreground hover:bg-secondary/60',
              ]"
              @click="selectedCopilotTarget = target.value"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm font-semibold">{{ target.label }}</span>
                <span
                  v-if="existingCopilotChannels[target.value]"
                  class="rounded border border-emerald-500/20 bg-emerald-500/10 px-1.5 py-0.5 text-[10px] text-emerald-700 dark:text-emerald-400"
                >
                  {{ t('subscription.channelExists') }}
                </span>
              </div>
              <p class="mt-1 text-xs text-muted-foreground">{{ target.description }}</p>
            </button>
          </div>
        </div>

        <div v-if="accountCount > 0" class="space-y-2">
          <Label class="text-xs text-muted-foreground">
            {{ t('subscription.accountsTitle', { target: selectedCopilotTargetOption.label, count: String(accountCount) }) }}
          </Label>
          <div class="space-y-2">
            <div
              v-for="account in copilotAccounts"
              :key="account.key"
              class="flex items-center justify-between gap-2 rounded-lg border border-border bg-background/70 px-3 py-2"
            >
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-foreground">
                  {{ account.name || t('subscription.accountUnnamed') }}
                </p>
                <p class="truncate font-mono text-xs text-muted-foreground">{{ maskKey(account.key) }}</p>
              </div>
              <div class="flex shrink-0 items-center gap-1">
                <template v-if="pendingRemoveKey === account.key">
                  <button
                    type="button"
                    class="inline-flex h-7 items-center gap-1 rounded border border-destructive/40 px-2 text-xs text-destructive transition-colors hover:bg-destructive/10 disabled:opacity-50"
                    :disabled="Boolean(removingKey)"
                    @click="removeAccountConfirmed(account.key)"
                  >
                    <Loader2 v-if="removingKey === account.key" class="h-3.5 w-3.5 animate-spin" />
                    <Check v-else class="h-3.5 w-3.5" />
                    {{ t('subscription.accountRemoveConfirm') }}
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-7 w-7 items-center justify-center rounded border border-border text-muted-foreground transition-colors hover:text-foreground"
                    :aria-label="t('common.cancel')"
                    @click="pendingRemoveKey = ''"
                  >
                    <X class="h-3.5 w-3.5" />
                  </button>
                </template>
                <button
                  v-else
                  type="button"
                  class="inline-flex h-7 w-7 items-center justify-center rounded border border-border text-muted-foreground transition-colors hover:border-destructive/40 hover:text-destructive disabled:opacity-50"
                  :title="t('subscription.accountRemove')"
                  :aria-label="t('subscription.accountRemove')"
                  :disabled="Boolean(removingKey) || copilotBusy"
                  @click="pendingRemoveKey = account.key"
                >
                  <Trash2 class="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="copilotUserCode" class="flex flex-wrap items-center gap-2 text-sm">
          <span class="text-muted-foreground">{{ t('copilotOAuth.userCode') }}</span>
          <code class="rounded bg-muted px-2 py-0.5 font-mono text-xs">{{ copilotUserCode }}</code>
          <button
            type="button"
            class="inline-flex h-6 w-6 items-center justify-center rounded border border-border text-muted-foreground transition-colors hover:text-foreground"
            :title="copilotUserCodeCopied ? t('common.copied') : t('common.copy')"
            :aria-label="copilotUserCodeCopied ? t('common.copied') : t('common.copy')"
            @click="copyCopilotUserCode"
          >
            <CheckCircle2 v-if="copilotUserCodeCopied" class="h-3.5 w-3.5 text-emerald-700 dark:text-emerald-400" />
            <Copy v-else class="h-3.5 w-3.5" />
          </button>
          <button type="button" class="text-xs text-primary underline" @click="openCopilotAuthorization">
            {{ t('copilotOAuth.openAuthorize') }}
          </button>
        </div>

        <template v-if="copilotOAuthError">
          <p class="text-xs text-destructive">{{ copilotOAuthError }}</p>
        </template>
        <template v-else-if="accountActionError || copilotCreateError || error">
          <p class="text-xs text-destructive">{{ accountActionError || copilotCreateError || error }}</p>
        </template>
        <template v-else-if="verifyingAccount">
          <p class="text-xs text-muted-foreground">{{ t('subscription.verifying') }}</p>
        </template>
        <template v-else-if="checkingCopilotChannel">
          <p class="text-xs text-muted-foreground">{{ t('subscription.checkingChannel') }}</p>
        </template>
        <template v-else-if="addingCopilotChannel">
          <p class="text-xs text-muted-foreground">{{ t('subscription.addingCopilotAccount') }}</p>
        </template>
        <template v-else-if="accountCount > 0">
          <p class="text-xs text-emerald-600">
            {{ t('subscription.accountsSummary', { target: selectedCopilotTargetOption.label, count: String(accountCount) }) }}
          </p>
        </template>
        <template v-else-if="availableCopilotToken">
          <p class="text-xs text-emerald-600">
            {{ t('subscription.copilotAuthorizationSavedOnly', { target: selectedCopilotTargetOption.label }) }}
          </p>
        </template>

        <div class="flex flex-wrap items-center gap-2">
          <Button :disabled="copilotBusy" @click="handleCopilotPrimaryAction">
            <Loader2 v-if="copilotBusy" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
            {{ copilotPrimaryActionLabel }}
          </Button>
          <button
            v-if="copilotPolling || copilotOAuthLoading"
            type="button"
            class="text-xs text-muted-foreground underline"
            @click="cancelCopilotAuthorization"
          >
            {{ t('copilotOAuth.cancel') }}
          </button>
          <button
            v-else-if="accountActionError && latestAuthorizedCopilotToken && !copilotBusy"
            type="button"
            class="text-xs text-primary underline"
            @click="retryAddAccount"
          >
            {{ t('subscription.retryAddAccount') }}
          </button>
        </div>

      </section>
    </div>
  </div>
</template>
