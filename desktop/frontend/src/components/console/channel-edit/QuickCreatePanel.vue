<script setup lang="ts">
import { Textarea } from '@/components/ui/textarea'
import { AlertCircle, CheckCircle2 } from 'lucide-vue-next'
import { useLanguage } from '@/composables/useLanguage'

defineProps<{
  quickInput: string
  detectedBaseUrls: string[]
  detectedApiKeys: string[]
  expectedRequestUrls: Array<{ baseUrl: string; expectedUrl: string }>
}>()

const emit = defineEmits<{
  (e: 'update:quick-input', value: string): void
  (e: 'quick-paste', text: string): void
}>()

const { tf } = useLanguage()
</script>

<template>
  <section class="space-y-4 p-6">
    <!-- 输入区域 -->
    <Textarea
      :model-value="quickInput"
      rows="10"
      class="!field-sizing-none min-h-[14rem] font-mono text-xs"
      :placeholder="tf('addChannel.quickInputPlaceholder', '粘贴配置片段，自动识别 Base URL 和 API Key（支持多行）\nhttps://api.example.com\nhttps://api-backup.example.com\nsk-ant-api03-xxxxxxxxxxxxxxxx\nsk-ant-api03-yyyyyyyyyyyyyyyy')"
      @update:model-value="(val) => emit('update:quick-input', val as string)"
      @paste="emit('quick-paste', $event.clipboardData?.getData('text/plain') || '')"
    />

    <!-- 检测结果 -->
    <div class="rounded-xl border border-border/60 bg-card/50 p-4">
      <div class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        {{ tf('addChannel.detectionStatus', '识别状态') }}
      </div>
      <div class="grid gap-3 md:grid-cols-2">
        <!-- Base URLs -->
        <div class="space-y-2 rounded-lg border border-border bg-background/70 p-3">
          <div class="flex items-center gap-2 text-xs font-semibold">
            <CheckCircle2 v-if="detectedBaseUrls.length" class="h-4 w-4 text-emerald-500" />
            <AlertCircle v-else class="h-4 w-4 text-muted-foreground" />
            Base URLs
          </div>
          <template v-if="detectedBaseUrls.length">
            <div v-for="item in expectedRequestUrls" :key="item.baseUrl" class="space-y-0.5">
              <p class="truncate text-[11px] font-medium text-emerald-600">{{ item.baseUrl }}</p>
              <p class="truncate text-[10px] text-muted-foreground">
                {{ tf('addChannel.expectedRequest', '预期请求') }}: {{ item.expectedUrl }}
              </p>
            </div>
          </template>
          <p v-else class="text-xs text-muted-foreground">
            {{ tf('addChannel.noneDetected', '未识别到 URL') }}
          </p>
        </div>

        <!-- API Keys -->
        <div class="space-y-2 rounded-lg border border-border bg-background/70 p-3">
          <div class="flex items-center gap-2 text-xs font-semibold">
            <CheckCircle2 v-if="detectedApiKeys.length" class="h-4 w-4 text-emerald-500" />
            <AlertCircle v-else class="h-4 w-4 text-muted-foreground" />
            {{ tf('channelEditor.auth.keys.label', 'API Keys') }}
          </div>
          <p v-if="detectedApiKeys.length" class="text-xs font-medium text-emerald-600">
            {{ detectedApiKeys.length }} {{ tf('channelCard.configuredKeys', 'active keys') }}
          </p>
          <p v-else class="text-xs text-muted-foreground">
            {{ tf('addChannel.noneDetected', '未识别到密钥') }}
          </p>
        </div>
      </div>
    </div>
  </section>
</template>
