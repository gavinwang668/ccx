<template>
  <v-card variant="outlined" rounded="lg" class="profile-changelog-timeline">
    <v-card-title class="d-flex align-center pa-3">
      <v-icon size="20" class="mr-2" color="primary">mdi-history</v-icon>
      <span class="text-body-1 font-weight-medium">{{ t('healthCenter.changelog.title') }}</span>
      <v-spacer />
      <v-chip
        v-if="status === 'open'"
        size="x-small"
        color="success"
        variant="tonal"
        density="comfortable"
      >
        <v-icon start size="10">mdi-circle</v-icon>
        {{ t('healthCenter.changelog.live') }}
      </v-chip>
      <v-chip
        v-else-if="status === 'connecting'"
        size="x-small"
        color="grey"
        variant="tonal"
        density="comfortable"
      >
        {{ t('healthCenter.changelog.connecting') }}
      </v-chip>
    </v-card-title>

    <v-alert
      v-if="status === 'closed'"
      type="warning"
      variant="tonal"
      density="compact"
      class="mx-3 mb-2"
      icon="mdi-wifi-off"
    >
      {{ t('healthCenter.changelog.disconnected') }}
    </v-alert>

    <v-divider />

    <div class="timeline-scroll">
      <div v-if="events.length === 0" class="pa-6 text-center text-medium-emphasis text-body-2">
        <v-icon size="24" class="mb-2 d-block mx-auto">mdi-clock-outline</v-icon>
        {{ t('healthCenter.changelog.empty') }}
      </div>

      <v-list v-else density="compact" class="pa-0">
        <v-list-item
          v-for="event in events"
          :key="event.eventUid"
          class="timeline-item"
        >
          <template #prepend>
            <v-icon :color="eventColor(event.eventType)" size="18">
              {{ eventIcon(event.eventType) }}
            </v-icon>
          </template>
          <v-list-item-title class="text-body-2">
            {{ eventTypeLabel(event.eventType) }}
            <span class="text-medium-emphasis">— {{ event.channelUid }}</span>
          </v-list-item-title>
          <v-list-item-subtitle class="text-caption">
            {{ event.summary }}
          </v-list-item-subtitle>
          <template #append>
            <span class="text-caption text-medium-emphasis">{{ relativeTime(event.createdAt) }}</span>
          </template>
        </v-list-item>
      </v-list>
    </div>
  </v-card>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from '../i18n'
import { connectProfileEvents, fetchProfileChangelog, type ProfileEventsConnectionStatus } from '../services/autopilot-events'
import type { ProfileChangeEvent } from '../services/api-types'

const { t } = useI18n()

// 最多渲染的事件数量：避免长时间挂着页面导致 DOM 无限增长
const MAX_RENDERED_EVENTS = 200

const events = ref<ProfileChangeEvent[]>([])
const status = ref<ProfileEventsConnectionStatus>('connecting')

let disconnect: (() => void) | null = null

function eventIcon(type: ProfileChangeEvent['eventType']): string {
  switch (type) {
    case 'health_changed':
      return 'mdi-heart-pulse'
    case 'discovery_completed':
      return 'mdi-radar'
    case 'auto_mapping_applied':
      return 'mdi-auto-fix'
    default:
      return 'mdi-swap-horizontal'
  }
}

function eventColor(type: ProfileChangeEvent['eventType']): string {
  switch (type) {
    case 'health_changed':
      return 'warning'
    case 'discovery_completed':
      return 'info'
    case 'auto_mapping_applied':
      return 'primary'
    default:
      return 'grey'
  }
}

function eventTypeLabel(type: ProfileChangeEvent['eventType']): string {
  switch (type) {
    case 'health_changed':
      return t('healthCenter.changelog.type.healthChanged')
    case 'discovery_completed':
      return t('healthCenter.changelog.type.discoveryCompleted')
    case 'auto_mapping_applied':
      return t('healthCenter.changelog.type.autoMappingApplied')
    default:
      return t('healthCenter.changelog.type.profileUpdated')
  }
}

function relativeTime(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ''
  const diffSec = Math.max(0, Math.floor((Date.now() - then) / 1000))
  if (diffSec < 60) return `${diffSec}s`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}h`
  return `${Math.floor(diffHour / 24)}d`
}

function prependEvent(event: ProfileChangeEvent) {
  events.value.unshift(event)
  if (events.value.length > MAX_RENDERED_EVENTS) {
    events.value = events.value.slice(0, MAX_RENDERED_EVENTS)
  }
}

async function loadHistory() {
  try {
    const resp = await fetchProfileChangelog({ limit: 50 })
    events.value = resp.events
  } catch {
    // 历史拉取失败不阻塞实时连接，保持空列表即可
  }
}

onMounted(() => {
  loadHistory()
  disconnect = connectProfileEvents({
    onEvent: prependEvent,
    onStatusChange: (s) => {
      status.value = s
    },
  })
})

onUnmounted(() => {
  disconnect?.()
})
</script>

<style scoped>
.timeline-scroll {
  max-height: 320px;
  overflow-y: auto;
}

.timeline-item {
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.timeline-item:last-child {
  border-bottom: none;
}
</style>
