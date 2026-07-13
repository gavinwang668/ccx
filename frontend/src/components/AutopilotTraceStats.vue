<template>
  <v-card variant="outlined" rounded="lg">
    <v-card-title class="d-flex align-center text-subtitle-1 font-weight-bold pb-0">
      <v-icon size="20" class="mr-2" color="info">mdi-chart-timeline-variant</v-icon>
      {{ t('autopilot.traceStats.title') }}
    </v-card-title>

    <v-card-text>
      <!-- 汇总数字 -->
      <v-row dense class="mb-3">
        <v-col cols="6" sm="4" md="2">
          <v-card variant="tonal" rounded="lg" class="pa-3 text-center">
            <div class="text-h5 font-weight-bold">{{ stats.totalCount }}</div>
            <div class="text-caption text-medium-emphasis">{{ t('autopilot.traceStats.total') }}</div>
          </v-card>
        </v-col>
        <v-col cols="6" sm="4" md="2">
          <v-card variant="tonal" rounded="lg" class="pa-3 text-center">
            <div class="text-h5 font-weight-bold">{{ stats.mismatchCount }}</div>
            <div class="text-caption text-medium-emphasis">{{ t('autopilot.traceStats.mismatches') }}</div>
          </v-card>
        </v-col>
        <v-col cols="6" sm="4" md="2">
          <v-card variant="tonal" :color="mismatchRateColor" rounded="lg" class="pa-3 text-center">
            <div class="text-h5 font-weight-bold">{{ mismatchRateDisplay }}</div>
            <div class="text-caption text-medium-emphasis">{{ t('autopilot.traceStats.mismatchRate') }}</div>
          </v-card>
        </v-col>
      </v-row>

      <!-- 模式分布 -->
      <div v-if="modeDistItems.length > 0" class="mb-3">
        <div class="text-caption text-medium-emphasis mb-2">{{ t('autopilot.traceStats.modeDistribution') }}</div>
        <div class="d-flex flex-wrap ga-2">
          <v-chip
            v-for="item in modeDistItems"
            :key="item.mode"
            size="small"
            variant="tonal"
            :color="modeColor(item.mode)"
          >
            {{ t(`autopilot.mode.${item.mode}`) || item.mode }}: {{ item.count }}
          </v-chip>
        </div>
      </div>

      <!-- TaskClass 分布 -->
      <div v-if="taskClassDistItems.length > 0">
        <div class="text-caption text-medium-emphasis mb-2">{{ t('autopilot.traceStats.taskClassDistribution') }}</div>
        <div class="d-flex flex-wrap ga-2">
          <v-chip
            v-for="item in taskClassDistItems"
            :key="item.taskClass"
            size="small"
            variant="outlined"
            color="secondary"
          >
            {{ item.taskClass }}: {{ item.count }}
          </v-chip>
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import type { AutopilotTraceStats } from '@/services/api-types'

const props = defineProps<{ stats: AutopilotTraceStats }>()
const { t } = useI18n()

const mismatchRateDisplay = computed(() => {
  if (props.stats.comparedCount === 0) return '-'
  return (props.stats.mismatchRate * 100).toFixed(1) + '%'
})

const mismatchRateColor = computed(() => {
  const rate = props.stats.mismatchRate
  if (props.stats.comparedCount === 0) return 'grey'
  if (rate <= 0.05) return 'success'
  if (rate <= 0.15) return 'warning'
  return 'error'
})

const modeDistItems = computed(() => {
  const dist = props.stats.modeDist
  if (!dist) return []
  return Object.entries(dist)
    .map(([mode, count]) => ({ mode, count }))
    .sort((a, b) => b.count - a.count)
})

const taskClassDistItems = computed(() => {
  const dist = props.stats.taskClassDist
  if (!dist) return []
  return Object.entries(dist)
    .map(([taskClass, count]) => ({ taskClass, count }))
    .sort((a, b) => b.count - a.count)
})

function modeColor(mode: string): string {
  const map: Record<string, string> = {
    off: 'grey',
    shadow: 'info',
    assist: 'warning',
    auto: 'success',
    active: 'primary',
    dry_run: 'info',
  }
  return map[mode] ?? 'grey'
}
</script>
