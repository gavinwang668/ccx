<template>
  <v-dialog
    :model-value="modelValue"
    max-width="520"
    :scrim="true"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <v-card rounded="xl">
      <v-card-title class="d-flex align-center justify-space-between pa-4">
        <div class="d-flex align-center ga-2">
          <v-icon color="primary">mdi-update</v-icon>
          <span>{{ t('update.title') }}</span>
        </div>
        <v-tooltip location="bottom" :text="t('app.actions.close') + ' (Esc)'" content-class="ccx-tooltip">
          <template #activator="{ props: tooltipProps }">
            <v-btn icon variant="text" v-bind="tooltipProps" @click="$emit('update:modelValue', false)">
              <v-icon>mdi-close</v-icon>
            </v-btn>
          </template>
        </v-tooltip>
      </v-card-title>

      <v-divider />

      <v-card-text class="pa-4">
        <div v-if="systemStore.isCheckingVersion" class="d-flex flex-column align-center py-6">
          <v-progress-circular indeterminate size="40" color="primary" />
          <p class="text-body-2 mt-3 text-medium-emphasis">{{ t('update.checking') }}</p>
        </div>

        <div v-else>
          <div class="d-flex justify-space-between align-center mb-3">
            <span class="text-body-2 text-medium-emphasis">{{ t('update.currentVersion') }}</span>
            <v-chip size="small" variant="outlined">{{ systemStore.versionInfo.currentVersion }}</v-chip>
          </div>

          <div v-if="systemStore.versionInfo.latestVersion" class="d-flex justify-space-between align-center mb-4">
            <span class="text-body-2 text-medium-emphasis">{{ t('update.latestVersion') }}</span>
            <v-chip size="small" :color="systemStore.versionInfo.hasUpdate ? 'success' : 'default'" variant="outlined">
              {{ systemStore.versionInfo.latestVersion }}
            </v-chip>
          </div>

          <v-alert v-if="systemStore.versionInfo.status === 'error'" type="error" variant="tonal" rounded="lg" class="mb-4">
            {{ t('update.checkFailed') }}
          </v-alert>

          <v-alert v-else-if="systemStore.versionInfo.hasUpdate" type="info" variant="tonal" rounded="lg" class="mb-4">
            {{ t('update.available') }}
          </v-alert>

          <v-alert v-else type="success" variant="tonal" rounded="lg" class="mb-4">
            {{ t('update.upToDate') }}
          </v-alert>
        </div>
      </v-card-text>

      <v-divider />

      <v-card-actions class="pa-4">
        <v-btn
          variant="outlined"
          :loading="systemStore.isCheckingVersion"
          @click="handleCheck"
        >
          {{ t('update.checkBtn') }}
        </v-btn>
        <v-spacer />
        <v-btn
          v-if="systemStore.versionInfo.releaseUrl"
          color="primary"
          variant="elevated"
          :href="systemStore.versionInfo.releaseUrl"
          target="_blank"
          rel="noopener"
        >
          {{ t('update.downloadBtn') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { useSystemStore } from '@/stores/system'
import { useI18n } from '@/i18n'

const { t } = useI18n()

defineProps<{ modelValue: boolean }>()
defineEmits<{ 'update:modelValue': [value: boolean] }>()

const systemStore = useSystemStore()

async function handleCheck() {
  window.dispatchEvent(new CustomEvent('ccx-check-version'))
}
</script>
