<template>
  <div class="model-capability-section">
    <v-card variant="outlined" rounded="lg">
      <v-card-title class="d-flex align-center justify-space-between pa-4 pb-2">
        <div class="d-flex align-center ga-2">
          <v-icon color="primary">mdi-database</v-icon>
          <span class="section-title">{{ t('addChannel.contextCapabilityTitle') }}</span>
        </div>
        <v-btn
          size="small"
          variant="tonal"
          color="primary"
          prepend-icon="mdi-refresh"
          :loading="fetchingModels"
          @click="$emit('sync-upstream')"
        >
          {{ t('addChannel.syncModelList') }}
        </v-btn>
      </v-card-title>

      <v-card-text class="pt-2">
        <div class="text-body-2 text-medium-emphasis mb-4">
          {{ t('addChannel.modelCapabilitiesRowsHint') }}
        </div>

        <v-row dense>
          <v-col cols="12" md="6">
            <v-text-field
              :model-value="defaultContextWindowTokens"
              :label="t('addChannel.defaultContextWindowLabel')"
              :hint="t('addChannel.defaultContextWindowHint')"
              type="number"
              min="0"
              prepend-inner-icon="mdi-database"
              persistent-hint
              variant="outlined"
              density="comfortable"
              @update:model-value="$emit('update:defaultContextWindowTokens', $event)"
            />
          </v-col>
          <v-col cols="12" md="6">
            <v-text-field
              :model-value="defaultMaxOutputTokens"
              :label="t('addChannel.defaultMaxOutputLabel')"
              :hint="t('addChannel.defaultMaxOutputHint')"
              type="number"
              min="0"
              prepend-inner-icon="mdi-text"
              persistent-hint
              variant="outlined"
              density="comfortable"
              @update:model-value="$emit('update:defaultMaxOutputTokens', $event)"
            />
          </v-col>
        </v-row>

        <div class="capability-container rounded-xl pa-3 mt-3">
          <div v-if="rows.length" class="d-flex flex-column ga-2">
            <div class="text-caption text-medium-emphasis d-flex align-center justify-space-between px-1">
              <span class="uppercase-label">{{ t('addChannel.modelCapabilitiesConfigured') }}</span>
              <v-chip size="x-small" variant="flat" color="primary" class="font-weight-bold px-2 font-mono">
                {{ rows.length }}
              </v-chip>
            </div>

            <div
              v-for="(row, index) in rows"
              :key="row.id"
              class="capability-item pa-3 rounded-lg"
            >
              <v-row dense align="center">
                <v-col cols="12" md="4">
                  <v-combobox
                    :model-value="row.model"
                    :items="targetModelOptions"
                    :loading="fetchingModels"
                    :label="t('addChannel.modelCapabilityModelLabel')"
                    placeholder="actual-model"
                    variant="outlined"
                    density="compact"
                    hide-details
                    clearable
                    eager
                    class="font-mono"
                    @focus="$emit('sync-upstream')"
                    @update:model-value="updateModel(index, $event)"
                    @update:menu="$emit('menu-update', $event)"
                  />
                </v-col>
                <v-col cols="6" md="2">
                  <v-text-field
                    :model-value="row.contextWindowTokens"
                    :label="t('addChannel.contextTokensShort')"
                    type="number"
                    min="0"
                    variant="outlined"
                    density="compact"
                    hide-details
                    @update:model-value="updateRow(index, { contextWindowTokens: $event })"
                  />
                </v-col>
                <v-col cols="6" md="2">
                  <v-text-field
                    :model-value="row.maxOutputTokens"
                    :label="t('addChannel.outputTokensShort')"
                    type="number"
                    min="0"
                    variant="outlined"
                    density="compact"
                    hide-details
                    @update:model-value="updateRow(index, { maxOutputTokens: $event })"
                  />
                </v-col>
                <v-col cols="6" md="2">
                  <v-combobox
                    :model-value="row.thinkingMode"
                    :items="thinkingModeOptions"
                    :label="t('addChannel.thinkingModeLabel')"
                    variant="outlined"
                    density="compact"
                    hide-details
                    clearable
                    eager
                    @update:model-value="updateRow(index, { thinkingMode: normalizeSelectableString($event) })"
                    @update:menu="$emit('menu-update', $event)"
                  />
                </v-col>
                <v-col cols="6" md="2" class="d-flex align-center ga-1">
                  <v-text-field
                    :model-value="row.reasoningEffortsText"
                    :label="t('addChannel.reasoningEffortsLabel')"
                    placeholder="high, max"
                    variant="outlined"
                    density="compact"
                    hide-details
                    @update:model-value="updateRow(index, { reasoningEffortsText: String($event || '') })"
                  />
                  <v-tooltip :text="t('app.actions.delete')" location="top" :open-delay="150">
                    <template #activator="{ props: tip }">
                      <v-btn
                        v-bind="tip"
                        size="small"
                        color="error"
                        icon
                        variant="text"
                        @click="removeRow(index)"
                      >
                        <v-icon size="16">mdi-close</v-icon>
                      </v-btn>
                    </template>
                  </v-tooltip>
                </v-col>
              </v-row>
              <div v-if="row.displayName || row.description" class="text-caption text-medium-emphasis mt-1 px-1">
                <span v-if="row.displayName" class="font-weight-medium">{{ row.displayName }}</span>
                <span v-if="row.displayName && row.description"> · </span>
                <span v-if="row.description">{{ row.description }}</span>
              </div>
              <div
                v-if="row.defaultOutputTokens || row.recommendedOutputTokens"
                class="text-caption text-medium-emphasis mt-1 px-1"
              >
                <span v-if="row.defaultOutputTokens">
                  {{ t('addChannel.defaultOutputTokensMeta', { tokens: formatTokens(row.defaultOutputTokens) }) }}
                </span>
                <span v-if="row.defaultOutputTokens && row.recommendedOutputTokens"> · </span>
                <span v-if="row.recommendedOutputTokens">
                  {{ t('addChannel.recommendedOutputTokensMeta', { tokens: formatTokens(row.recommendedOutputTokens) }) }}
                </span>
              </div>
              <v-row dense class="mt-1">
                <v-col cols="12" sm="4" md="3">
                  <v-text-field
                    :model-value="row.inputCacheHitPrice"
                    :label="t('addChannel.inputCacheHitPriceLabel')"
                    type="number"
                    min="0"
                    step="0.000001"
                    variant="outlined"
                    density="compact"
                    hide-details
                    @update:model-value="updateRow(index, { inputCacheHitPrice: $event })"
                  />
                </v-col>
                <v-col cols="12" sm="4" md="3">
                  <v-text-field
                    :model-value="row.inputCacheMissPrice"
                    :label="t('addChannel.inputCacheMissPriceLabel')"
                    type="number"
                    min="0"
                    step="0.000001"
                    variant="outlined"
                    density="compact"
                    hide-details
                    @update:model-value="updateRow(index, { inputCacheMissPrice: $event })"
                  />
                </v-col>
                <v-col cols="12" sm="4" md="3">
                  <v-text-field
                    :model-value="row.outputPrice"
                    :label="t('addChannel.outputPriceLabel')"
                    type="number"
                    min="0"
                    step="0.000001"
                    variant="outlined"
                    density="compact"
                    hide-details
                    @update:model-value="updateRow(index, { outputPrice: $event })"
                  />
                </v-col>
                <v-col cols="12" md="3" class="d-flex align-center text-caption text-medium-emphasis">
                  {{ t('addChannel.modelPricingUnitHint') }}
                </v-col>
              </v-row>
              <div v-if="row.source === 'builtin' && row.matchedPattern" class="text-caption text-primary mt-1 px-1">
                {{ t('addChannel.modelCapabilityBuiltinMatched', { pattern: row.matchedPattern }) }}
              </div>
            </div>
          </div>

          <div class="add-capability-row d-flex align-center ga-3 pa-3 mt-3 rounded-lg">
            <v-combobox
              v-model="newModel"
              :items="targetModelOptions"
              :loading="fetchingModels"
              :label="t('addChannel.modelCapabilityModelLabel')"
              :placeholder="t('addChannel.modelCapabilityModelPlaceholder')"
              variant="outlined"
              density="compact"
              hide-details
              clearable
              eager
              class="flex-grow-1 font-mono"
              @focus="$emit('sync-upstream')"
              @update:menu="$emit('menu-update', $event)"
              @keyup.enter="addRow"
            />
            <v-btn
              color="primary"
              height="40"
              variant="flat"
              class="rounded-lg px-4"
              :disabled="!newModelName"
              @click="addRow"
            >
              <v-icon size="18" class="mr-1">mdi-plus</v-icon>
              {{ t('app.actions.add') }}
            </v-btn>
          </div>
        </div>

        <div v-if="error" class="text-error text-caption mt-2">
          {{ error }}
        </div>
        <div v-if="fetchModelsError" class="text-error text-caption mt-2">
          {{ fetchModelsError }}
        </div>

        <v-switch
          :model-value="allowUnknownContext"
          :label="t('addChannel.allowUnknownContextLabel')"
          :hint="t('addChannel.allowUnknownContextHint')"
          color="primary"
          inset
          persistent-hint
          hide-details="auto"
          class="mt-2"
          @update:model-value="$emit('update:allowUnknownContext', !!$event)"
        />
      </v-card-text>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '../../i18n'
import {
  capabilityRowDefaultsFromBuiltin,
  createModelCapabilityRow,
  normalizeSelectableString,
  resolveBuiltinUpstreamModelCapability,
  type ModelCapabilityRow,
} from '../../utils/channelPayload'

type SelectableString = string | { title?: string; value?: unknown } | null | undefined
type ModelOptionValue = string | { title: string; value: string } | null

const props = defineProps<{
  rows: ModelCapabilityRow[]
  targetModelOptions: Array<{ title: string; value: string }>
  fetchingModels: boolean
  fetchModelsError: string
  error: string
  defaultContextWindowTokens: string | number | null
  defaultMaxOutputTokens: string | number | null
  allowUnknownContext: boolean
}>()

const emit = defineEmits<{
  'update:rows': [ModelCapabilityRow[]]
  'update:defaultContextWindowTokens': [value: string | number | null]
  'update:defaultMaxOutputTokens': [value: string | number | null]
  'update:allowUnknownContext': [value: boolean]
  'sync-upstream': []
  'menu-update': [open: boolean]
}>()

const { t } = useI18n()
const newModel = ref<ModelOptionValue>('')
const thinkingModeOptions = ['thinking', 'extended', 'adaptive', 'adaptive_only', 'adaptive_always_on']
const newModelName = computed(() => normalizeSelectableString(newModel.value).trim())

function formatTokens(value?: number) {
  if (!value) return ''
  if (value >= 1000 && value % 1000 === 0) return `${value / 1000}k`
  return value.toLocaleString()
}

const nextRowId = () => Date.now() + Math.floor(Math.random() * 1000)

function updateRows(rows: ModelCapabilityRow[]) {
  emit('update:rows', rows)
}

function updateRow(index: number, patch: Partial<ModelCapabilityRow>) {
  const rows = props.rows.map((row, rowIndex) => (
    rowIndex === index ? { ...row, ...patch, source: patch.source || row.source } : row
  ))
  updateRows(rows)
}

function updateModel(index: number, value: SelectableString) {
  const model = normalizeSelectableString(value).trim()
  const builtin = resolveBuiltinUpstreamModelCapability(model)
  updateRow(index, {
    model,
    ...(builtin ? capabilityRowDefaultsFromBuiltin(builtin.capability) : {}),
    source: builtin ? 'builtin' : 'custom',
    matchedPattern: builtin?.pattern || '',
  })
}

function addRow() {
  const model = newModelName.value
  if (!model) return
  const builtin = resolveBuiltinUpstreamModelCapability(model)
  const row = createModelCapabilityRow(
    nextRowId(),
    model,
    builtin?.capability,
    builtin ? 'builtin' : 'custom',
    builtin?.pattern || '',
  )
  updateRows([...props.rows, row])
  newModel.value = ''
}

function removeRow(index: number) {
  updateRows(props.rows.filter((_, rowIndex) => rowIndex !== index))
}
</script>

<style scoped>
.section-title {
  font-size: 1.125rem;
  font-weight: 600;
}

.font-mono {
  font-family: 'SF Mono', 'Fira Code', Monaco, Consolas, monospace !important;
}

.capability-container {
  background: rgba(var(--v-border-color), 0.03);
  border: 1px solid rgba(var(--v-border-color), 0.08);
}

.capability-item {
  background: rgb(var(--v-theme-surface));
  border: 1px solid rgba(var(--v-border-color), 0.12);
}

.add-capability-row {
  background: rgba(var(--v-theme-surface), 0.8);
  border: 1px solid rgba(var(--v-border-color), 0.15);
}

.uppercase-label {
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 600;
}
</style>
