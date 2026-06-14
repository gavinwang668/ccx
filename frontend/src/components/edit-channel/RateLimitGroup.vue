<template>
  <v-card variant="outlined" class="pa-4">
    <div class="text-caption font-weight-bold text-uppercase text-medium-emphasis mb-3">
      <v-icon size="small" color="primary" class="mr-1">mdi-speedometer</v-icon>
      {{ t('channelEditor.rateLimit.title') }}
    </div>

    <span class="text-caption text-medium-emphasis mb-3 d-block">{{ t('channelEditor.rateLimit.title') }}</span>

    <v-row dense>
      <v-col cols="12" md="4">
        <v-text-field
          :model-value="form.rateLimitRpm"
          :label="t('channelEditor.rateLimit.rpm.label')"
          :placeholder="t('channelEditor.rateLimit.rpm.placeholder')"
          prepend-inner-icon="mdi-speedometer"
          :hint="t('channelEditor.rateLimit.rpm.hint')"
          persistent-hint
          clearable
          variant="outlined"
          density="comfortable"
          type="number"
          min="1"
          @update:model-value="updateField('rateLimitRpm', $event ? parseInt($event) : null)"
        />
      </v-col>
      <v-col cols="12" md="4">
        <v-text-field
          :model-value="form.rateLimitWindowMinutes"
          :label="t('channelEditor.rateLimit.window.label')"
          :placeholder="t('channelEditor.rateLimit.window.placeholder')"
          prepend-inner-icon="mdi-clock-outline"
          :hint="t('channelEditor.rateLimit.window.hint')"
          persistent-hint
          clearable
          variant="outlined"
          density="comfortable"
          type="number"
          min="1"
          @update:model-value="updateField('rateLimitWindowMinutes', $event ? parseInt($event) : null)"
        />
      </v-col>
      <v-col cols="12" md="4">
        <v-text-field
          :model-value="form.rateLimitMaxConcurrent"
          :label="t('channelEditor.rateLimit.maxConcurrent.label')"
          :placeholder="t('channelEditor.rateLimit.maxConcurrent.placeholder')"
          prepend-inner-icon="mdi-server-network"
          :hint="t('channelEditor.rateLimit.maxConcurrent.hint')"
          persistent-hint
          clearable
          variant="outlined"
          density="comfortable"
          type="number"
          min="1"
          @update:model-value="updateField('rateLimitMaxConcurrent', $event ? parseInt($event) : null)"
        />
      </v-col>
    </v-row>
  </v-card>
</template>

<script setup lang="ts">
import { useI18n } from '../../i18n'

interface FormData {
  rateLimitRpm: string | number | null
  rateLimitWindowMinutes: string | number | null
  rateLimitMaxConcurrent: string | number | null
}

interface Props {
  form: FormData
}

defineProps<Props>()

const emit = defineEmits<{
  'update:field': [field: keyof FormData, value: unknown]
}>()

const { t } = useI18n()

const updateField = (field: keyof FormData, value: unknown) => {
  emit('update:field', field, value)
}
</script>
