<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import type { ChannelStatus } from '@/services/admin-api'

interface Props {
  status?: ChannelStatus | 'healthy' | 'error' | 'unknown' | ''
  circuitState?: string
}

const props = defineProps<Props>()

const variant = computed(() => {
  const s = props.status || 'active'
  if (s === 'disabled') return 'destructive'
  if (props.circuitState === 'open') return 'destructive'
  if (s === 'suspended') return 'secondary'
  if (s === 'error') return 'destructive'
  return 'default'
})

const label = computed(() => {
  const s = props.status || 'active'
  if (props.circuitState === 'open') return 'Circuit Open'
  if (s === 'suspended') return 'Paused'
  return s.charAt(0).toUpperCase() + s.slice(1)
})

const dotClass = computed(() => {
  const s = props.status || 'active'
  if (props.circuitState === 'open') return 'bg-rose-500 animate-pulse'
  if (s === 'disabled' || s === 'error') return 'bg-rose-500'
  if (s === 'suspended') return 'bg-amber-500'
  return 'bg-emerald-500'
})
</script>

<template>
  <Badge :variant="variant" class="gap-1.5 text-[10px]">
    <span class="w-1.5 h-1.5 rounded-full" :class="dotClass" />
    {{ label }}
  </Badge>
</template>
