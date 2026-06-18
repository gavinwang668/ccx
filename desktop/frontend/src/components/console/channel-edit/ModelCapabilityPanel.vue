<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Database, Plus, RefreshCw, Trash2 } from 'lucide-vue-next'
import { useLanguage } from '@/composables/useLanguage'
import {
  capabilityRowDefaultsFromBuiltin,
  createModelCapabilityRow,
  resolveBuiltinUpstreamModelCapability,
  type ModelCapabilityRow,
} from '@/utils/channel-payload'

const props = defineProps<{
  rows: ModelCapabilityRow[]
  targetModels: string[]
  fetchingModels: boolean
  fetchModelsError: string
  error: string
  defaultContextWindowTokens: string | number
  defaultMaxOutputTokens: string | number
  allowUnknownContext: boolean
}>()

const emit = defineEmits<{
  'update:rows': [rows: ModelCapabilityRow[]]
  'update:defaultContextWindowTokens': [value: string | number]
  'update:defaultMaxOutputTokens': [value: string | number]
  'update:allowUnknownContext': [value: boolean]
  'sync-upstream-models': []
}>()

const { t, tf } = useLanguage()
const newModel = ref('')
const thinkingModeOptions = ['thinking', 'extended', 'adaptive', 'adaptive_only', 'adaptive_always_on']
const newModelName = computed(() => newModel.value.trim())
const datalistId = `model-capability-models-${Math.random().toString(36).slice(2)}`
const thinkingDatalistId = `model-capability-thinking-${Math.random().toString(36).slice(2)}`

function updateRows(rows: ModelCapabilityRow[]) {
  emit('update:rows', rows)
}

function updateRow(id: number, patch: Partial<ModelCapabilityRow>) {
  updateRows(props.rows.map(row => row.id === id ? { ...row, ...patch } : row))
}

function updateModel(row: ModelCapabilityRow, model: string) {
  const nextModel = model.trim()
  const builtin = resolveBuiltinUpstreamModelCapability(nextModel)
  updateRow(row.id, {
    model: nextModel,
    ...(builtin ? capabilityRowDefaultsFromBuiltin(builtin.capability) : {}),
    source: builtin ? 'builtin' : 'custom',
    matchedPattern: builtin?.pattern || '',
  })
}

function formatTokens(value?: number) {
  if (!value) return ''
  if (value >= 1000 && value % 1000 === 0) return `${value / 1000}k`
  return value.toLocaleString()
}

function addRow() {
  const model = newModelName.value
  if (!model) return
  const builtin = resolveBuiltinUpstreamModelCapability(model)
  updateRows([
    ...props.rows,
    createModelCapabilityRow(
      Date.now() + Math.floor(Math.random() * 1000),
      model,
      builtin?.capability,
      builtin ? 'builtin' : 'custom',
      builtin?.pattern || '',
    ),
  ])
  newModel.value = ''
}

function removeRow(id: number) {
  updateRows(props.rows.filter(row => row.id !== id))
}
</script>

<template>
  <section class="space-y-4 rounded-xl border border-border/60 bg-background/60 p-4 shadow-sm backdrop-blur-sm">
    <div class="flex items-center justify-between gap-3 border-b border-border/40 pb-2">
      <div class="min-w-0 space-y-1">
        <div class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-primary">
          <Database class="h-3 w-3" />
          {{ t('addChannel.contextCapabilityTitle') }}
        </div>
        <p class="text-[10px] leading-4 text-muted-foreground">
          {{ t('addChannel.modelCapabilitiesRowsHint') }}
        </p>
      </div>
      <Button
        type="button"
        variant="outline"
        size="sm"
        class="h-8 shrink-0 px-3 text-[10px]"
        @click="emit('sync-upstream-models')"
      >
        <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': fetchingModels }" />
        {{ t('addChannel.syncModelList') }}
      </Button>
    </div>

    <div class="grid gap-3 md:grid-cols-2">
      <div class="space-y-1.5">
        <Label class="text-xs font-semibold text-muted-foreground">
          {{ t('addChannel.defaultContextWindowLabel') }}
        </Label>
        <Input
          :model-value="defaultContextWindowTokens"
          type="number"
          min="0"
          class="h-9"
          :placeholder="tf('addChannel.defaultContextWindowLabel', '默认上下文窗口 tokens')"
          @update:model-value="(val) => emit('update:defaultContextWindowTokens', val as string | number)"
        />
        <p class="text-[10px] leading-4 text-muted-foreground">
          {{ t('addChannel.defaultContextWindowHint') }}
        </p>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs font-semibold text-muted-foreground">
          {{ t('addChannel.defaultMaxOutputLabel') }}
        </Label>
        <Input
          :model-value="defaultMaxOutputTokens"
          type="number"
          min="0"
          class="h-9"
          :placeholder="tf('addChannel.defaultMaxOutputLabel', '默认最大输出 tokens')"
          @update:model-value="(val) => emit('update:defaultMaxOutputTokens', val as string | number)"
        />
        <p class="text-[10px] leading-4 text-muted-foreground">
          {{ t('addChannel.defaultMaxOutputHint') }}
        </p>
      </div>
    </div>

    <datalist :id="datalistId">
      <option v-for="model in targetModels" :key="model" :value="model" />
    </datalist>
    <datalist :id="thinkingDatalistId">
      <option v-for="mode in thinkingModeOptions" :key="mode" :value="mode" />
    </datalist>

    <div v-if="rows.length" class="space-y-2.5">
      <div class="flex items-center justify-between px-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground/60">
        <span>{{ t('addChannel.modelCapabilitiesConfigured') }}</span>
        <span class="rounded-full border border-primary/20 bg-primary/10 px-2 py-0.5 font-mono text-primary">{{ rows.length }}</span>
      </div>
      <div
        v-for="row in rows"
        :key="row.id"
        class="grid gap-2 rounded-lg border border-border/60 bg-background/60 p-3 shadow-2xs md:grid-cols-[minmax(0,1.5fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,0.9fr)_minmax(0,1fr)_auto]"
      >
        <div class="space-y-1">
          <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.modelCapabilityModelLabel') }}</Label>
          <Input
            :model-value="row.model"
            :list="datalistId"
            class="h-8 font-mono text-xs"
            placeholder="actual-model"
            @focus="emit('sync-upstream-models')"
            @update:model-value="(val) => updateModel(row, String(val || ''))"
          />
          <p v-if="row.source === 'builtin' && row.matchedPattern" class="truncate text-[10px] text-primary">
            {{ t('addChannel.modelCapabilityBuiltinMatched', { pattern: row.matchedPattern }) }}
          </p>
        </div>
        <div class="space-y-1">
          <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.contextTokensShort') }}</Label>
          <Input
            :model-value="row.contextWindowTokens ?? ''"
            type="number"
            min="0"
            class="h-8 text-xs"
            @update:model-value="(val) => updateRow(row.id, { contextWindowTokens: val as string | number })"
          />
        </div>
        <div class="space-y-1">
          <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.outputTokensShort') }}</Label>
          <Input
            :model-value="row.maxOutputTokens ?? ''"
            type="number"
            min="0"
            class="h-8 text-xs"
            @update:model-value="(val) => updateRow(row.id, { maxOutputTokens: val as string | number })"
          />
        </div>
        <div class="space-y-1">
          <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.thinkingModeLabel') }}</Label>
          <Input
            :model-value="row.thinkingMode"
            :list="thinkingDatalistId"
            class="h-8 font-mono text-xs"
            @update:model-value="(val) => updateRow(row.id, { thinkingMode: String(val || '') })"
          />
        </div>
        <div class="space-y-1">
          <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.reasoningEffortsLabel') }}</Label>
          <Input
            :model-value="row.reasoningEffortsText"
            class="h-8 font-mono text-xs"
            placeholder="high, max"
            @update:model-value="(val) => updateRow(row.id, { reasoningEffortsText: String(val || '') })"
          />
        </div>
        <div class="flex items-end">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            class="h-8 w-8 text-destructive hover:bg-destructive/10"
            @click="removeRow(row.id)"
          >
            <Trash2 class="h-3.5 w-3.5" />
          </Button>
        </div>
        <p v-if="row.displayName || row.description" class="md:col-span-6 text-[10px] leading-4 text-muted-foreground">
          <span v-if="row.displayName" class="font-semibold">{{ row.displayName }}</span>
          <span v-if="row.displayName && row.description"> · </span>
          <span v-if="row.description">{{ row.description }}</span>
        </p>
        <p
          v-if="row.defaultOutputTokens || row.recommendedOutputTokens"
          class="md:col-span-6 text-[10px] leading-4 text-muted-foreground"
        >
          <span v-if="row.defaultOutputTokens">
            {{ t('addChannel.defaultOutputTokensMeta', { tokens: formatTokens(row.defaultOutputTokens) }) }}
          </span>
          <span v-if="row.defaultOutputTokens && row.recommendedOutputTokens"> · </span>
          <span v-if="row.recommendedOutputTokens">
            {{ t('addChannel.recommendedOutputTokensMeta', { tokens: formatTokens(row.recommendedOutputTokens) }) }}
          </span>
        </p>
        <div class="grid gap-2 md:col-span-6 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_auto] md:items-end">
          <div class="space-y-1">
            <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.inputCacheHitPriceLabel') }}</Label>
            <Input
              :model-value="row.inputCacheHitPrice ?? ''"
              type="number"
              min="0"
              step="0.000001"
              class="h-8 text-xs"
              @update:model-value="(val) => updateRow(row.id, { inputCacheHitPrice: val as string | number })"
            />
          </div>
          <div class="space-y-1">
            <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.inputCacheMissPriceLabel') }}</Label>
            <Input
              :model-value="row.inputCacheMissPrice ?? ''"
              type="number"
              min="0"
              step="0.000001"
              class="h-8 text-xs"
              @update:model-value="(val) => updateRow(row.id, { inputCacheMissPrice: val as string | number })"
            />
          </div>
          <div class="space-y-1">
            <Label class="text-[10px] font-semibold uppercase text-muted-foreground/70">{{ t('addChannel.outputPriceLabel') }}</Label>
            <Input
              :model-value="row.outputPrice ?? ''"
              type="number"
              min="0"
              step="0.000001"
              class="h-8 text-xs"
              @update:model-value="(val) => updateRow(row.id, { outputPrice: val as string | number })"
            />
          </div>
          <p class="pb-2 text-[10px] leading-4 text-muted-foreground">
            {{ t('addChannel.modelPricingUnitHint') }}
          </p>
        </div>
      </div>
    </div>

    <div class="grid gap-2 rounded-lg border border-dashed border-primary/30 bg-primary/[0.03] p-3 md:grid-cols-[1fr_auto] md:items-end">
      <div class="space-y-1">
        <Label class="text-xs font-semibold text-muted-foreground">
          {{ t('addChannel.modelCapabilityModelLabel') }}
        </Label>
        <Input
          v-model="newModel"
          :list="datalistId"
          class="h-9 font-mono text-xs"
          :placeholder="t('addChannel.modelCapabilityModelPlaceholder')"
          @focus="emit('sync-upstream-models')"
          @keydown.enter.prevent="addRow"
        />
      </div>
      <Button
        type="button"
        variant="outline"
        size="sm"
        class="h-9 justify-self-start px-3.5 md:justify-self-auto"
        :disabled="!newModelName"
        @click="addRow"
      >
        <Plus class="h-4 w-4" />
      </Button>
    </div>

    <p v-if="error" class="text-[10px] leading-4 text-destructive">
      {{ error }}
    </p>
    <p v-if="fetchModelsError" class="text-[10px] leading-4 text-destructive">
      {{ fetchModelsError }}
    </p>

    <div class="flex items-center justify-between gap-3 rounded-lg border border-border/50 bg-background/80 p-3">
      <div class="min-w-0 space-y-0.5">
        <Label class="text-xs font-medium">{{ t('addChannel.allowUnknownContextLabel') }}</Label>
        <p class="text-[10px] leading-4 text-muted-foreground">{{ t('addChannel.allowUnknownContextHint') }}</p>
      </div>
      <Switch :model-value="allowUnknownContext" @update:model-value="emit('update:allowUnknownContext', !!$event)" />
    </div>
  </section>
</template>
