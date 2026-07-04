<template>
  <div class="embedding-compatibility-section">
    <v-card variant="outlined" rounded="lg">
      <v-card-title class="d-flex align-center pa-4 pb-2">
        <div class="d-flex align-center ga-2">
          <v-icon color="primary">mdi-vector-polyline</v-icon>
          <span class="section-title">{{ t('addChannel.embeddingCompatibilityTitle') }}</span>
        </div>
      </v-card-title>

      <v-card-text class="pt-2">
        <div class="text-body-2 text-medium-emphasis mb-4">
          {{ t('addChannel.embeddingCompatibilityHint') }}
        </div>

        <div v-if="mappedTargetModels.length" class="d-flex align-center flex-wrap ga-2 mb-3">
          <span class="text-caption text-medium-emphasis">{{ t('addChannel.embeddingCompatibilityRedirectTargets') }}</span>
          <v-chip
            v-for="model in mappedTargetModels"
            :key="model"
            size="x-small"
            variant="tonal"
            color="primary"
            class="font-mono"
          >
            {{ model }}
          </v-chip>
        </div>

        <div class="compatibility-container rounded-xl pa-3 mt-3">
          <div v-if="rows.length" class="d-flex flex-column ga-3">
            <div class="text-caption text-medium-emphasis d-flex align-center justify-space-between px-1">
              <span class="uppercase-label">{{ t('addChannel.embeddingCompatibilityConfigured') }}</span>
              <v-chip size="x-small" variant="flat" color="primary" class="font-weight-bold px-2 font-mono">
                {{ rows.length }}
              </v-chip>
            </div>

            <div
              v-for="(row, index) in rows"
              :key="row.id"
              class="compatibility-card rounded-lg overflow-hidden"
            >
              <div class="compatibility-card-header d-flex align-center justify-space-between ga-3 px-3 py-2">
                <div class="d-flex align-center ga-2 min-width-0">
                  <div class="model-avatar flex-shrink-0">{{ modelInitial(row.model) }}</div>
                  <div class="font-mono text-body-2 font-weight-bold text-truncate">
                    {{ row.model || t('addChannel.embeddingCapabilityModelPlaceholder') }}
                  </div>
                </div>
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
              </div>

              <div class="compatibility-card-body pa-3">
                <v-row dense>
                  <v-col cols="12" md="4">
                    <v-combobox
                      :model-value="row.model"
                      :items="targetModelOptions"
                      item-title="title"
                      item-value="value"
                      :label="t('addChannel.embeddingCapabilityModelLabel')"
                      :placeholder="t('addChannel.embeddingCapabilityModelPlaceholder')"
                      variant="outlined"
                      density="compact"
                      hide-details
                      clearable
                      eager
                      class="font-mono"
                      @focus="$emit('sync-upstream')"
                      @update:model-value="updateRow(index, { model: normalizeSelectableString($event) })"
                      @update:menu="$emit('menu-update', $event)"
                    />
                  </v-col>
                  <v-col cols="12" md="4">
                    <v-text-field
                      :model-value="row.embeddingSpaceId"
                      :label="t('addChannel.embeddingSpaceIdLabel')"
                      :placeholder="t('addChannel.embeddingSpaceIdPlaceholder')"
                      variant="outlined"
                      density="compact"
                      hide-details
                      @update:model-value="updateRow(index, { embeddingSpaceId: String($event || '') })"
                    />
                  </v-col>
                  <v-col cols="6" md="2">
                    <v-text-field
                      :model-value="row.dimensions"
                      :label="t('addChannel.embeddingDimensionsLabel')"
                      type="number"
                      min="1"
                      step="1"
                      variant="outlined"
                      density="compact"
                      hide-details
                      @update:model-value="updateRow(index, { dimensions: $event })"
                    />
                  </v-col>
                  <v-col cols="6" md="2">
                    <v-select
                      :model-value="row.normalized"
                      :items="normalizedOptions"
                      :label="t('addChannel.embeddingNormalizedLabel')"
                      variant="outlined"
                      density="compact"
                      hide-details
                      @update:model-value="updateRow(index, { normalized: $event as EmbeddingCapabilityRow['normalized'] })"
                      @update:menu="$emit('menu-update', $event)"
                    />
                  </v-col>
                  <v-col cols="12">
                    <v-text-field
                      :model-value="row.supportedDimensionsText"
                      :label="t('addChannel.embeddingSupportedDimensionsLabel')"
                      :placeholder="t('addChannel.embeddingSupportedDimensionsPlaceholder')"
                      variant="outlined"
                      density="compact"
                      hide-details
                      @update:model-value="updateRow(index, { supportedDimensionsText: String($event || '') })"
                    />
                  </v-col>
                </v-row>
              </div>
            </div>
          </div>

          <div class="add-compatibility-row d-flex align-center ga-3 pa-3 mt-3 rounded-lg">
            <v-combobox
              :model-value="newModel"
              :items="targetModelOptions"
              item-title="title"
              item-value="value"
              :loading="fetchingModels"
              :label="t('addChannel.embeddingCapabilityModelLabel')"
              :placeholder="t('addChannel.embeddingCapabilityModelPlaceholder')"
              variant="outlined"
              density="compact"
              hide-details
              clearable
              eager
              class="flex-grow-1 font-mono"
              @focus="$emit('sync-upstream')"
              @update:model-value="handleNewModelUpdate"
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
      </v-card-text>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '../../i18n'
import {
  createEmbeddingCapabilityRow,
  normalizeSelectableString,
  type EmbeddingCapabilityRow,
} from '../../utils/channelPayload'

type ModelOptionValue = string | { title: string; value: string } | null

const props = defineProps<{
  rows: EmbeddingCapabilityRow[]
  targetModelOptions: Array<{ title: string; value: string }>
  mappedTargetModels: string[]
  fetchingModels: boolean
  fetchModelsError: string
  error: string
}>()

const emit = defineEmits<{
  'update:rows': [EmbeddingCapabilityRow[]]
  'sync-upstream': []
  'menu-update': [open: boolean]
}>()

const { t } = useI18n()
const newModel = ref<ModelOptionValue>('')
const newModelName = computed(() => normalizeSelectableString(newModel.value).trim())
const normalizedOptions = computed(() => [
  { title: t('addChannel.embeddingNormalizedUnknown'), value: '' },
  { title: t('addChannel.embeddingNormalizedTrue'), value: 'true' },
  { title: t('addChannel.embeddingNormalizedFalse'), value: 'false' },
])

const nextRowId = () => Date.now() + Math.floor(Math.random() * 1000)

function updateRows(rows: EmbeddingCapabilityRow[]) {
  emit('update:rows', rows)
}

function updateRow(index: number, patch: Partial<EmbeddingCapabilityRow>) {
  updateRows(props.rows.map((row, rowIndex) => (rowIndex === index ? { ...row, ...patch } : row)))
}

function addRow() {
  const model = newModelName.value
  if (!model) return
  if (props.rows.some(row => normalizeSelectableString(row.model).trim().toLowerCase() === model.toLowerCase())) {
    newModel.value = ''
    return
  }
  updateRows([...props.rows, createEmbeddingCapabilityRow(nextRowId(), model)])
  newModel.value = ''
}

function handleNewModelUpdate(value: ModelOptionValue) {
  newModel.value = value
  const model = normalizeSelectableString(value).trim()
  const selectedKnownModel = props.targetModelOptions.some(option => option.value.trim().toLowerCase() === model.toLowerCase())
  if (selectedKnownModel) {
    addRow()
  }
}

function removeRow(index: number) {
  updateRows(props.rows.filter((_, rowIndex) => rowIndex !== index))
}

function modelInitial(model: string) {
  const source = (model || '?').trim()
  return source ? source.slice(0, 1).toUpperCase() : '?'
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

.compatibility-container {
  background: rgba(var(--v-border-color), 0.03);
  border: 1px solid rgba(var(--v-border-color), 0.08);
}

.compatibility-card {
  background: rgb(var(--v-theme-surface));
  border: 1px solid rgba(var(--v-border-color), 0.12);
}

.compatibility-card-header {
  background: rgba(var(--v-border-color), 0.035);
  border-bottom: 1px solid rgba(var(--v-border-color), 0.08);
}

.model-avatar {
  align-items: center;
  background: rgba(var(--v-theme-primary), 0.12);
  border: 1px solid rgba(var(--v-theme-primary), 0.2);
  border-radius: 999px;
  color: rgb(var(--v-theme-primary));
  display: inline-flex;
  font-size: 0.75rem;
  font-weight: 700;
  height: 28px;
  justify-content: center;
  width: 28px;
}

.add-compatibility-row {
  background: rgba(var(--v-theme-surface), 0.8);
  border: 1px solid rgba(var(--v-border-color), 0.15);
}

.uppercase-label {
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.min-width-0 {
  min-width: 0;
}
</style>
