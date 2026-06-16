<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Eye, EyeOff, Zap } from 'lucide-vue-next'
import { useLanguage } from '@/composables/useLanguage'

defineProps<{
  channelType: string
  isEditMode: boolean
  noVision: boolean
  saving: boolean
  serviceType?: string
  serviceTypeOptions?: Array<{ label: string; value: string }>
}>()

const emit = defineEmits<{
  (e: 'toggle-no-vision'): void
  (e: 'test-capability'): void
  (e: 'update:service-type', value: string): void
}>()

const { tf } = useLanguage()
</script>

<template>
  <div class="flex shrink-0 items-start justify-between gap-3 border-b border-border/60 bg-card/50 p-5 backdrop-blur-sm">
    <div class="min-w-0 space-y-1">
      <div class="text-[10px] font-bold uppercase tracking-[0.2em] text-primary/80">
        {{ channelType }} CHANNEL
      </div>
      <h3 class="text-xl font-bold tracking-tight">
        {{ isEditMode
          ? tf('channelEditor.title.edit', '编辑渠道')
          : tf('channelEditor.title.create', '添加渠道')
        }}
      </h3>
    </div>

    <!-- 创建模式：显示上游类型选择器 -->
    <div v-if="!isEditMode && serviceTypeOptions" class="flex shrink-0 items-center gap-3">
      <span class="text-xs font-medium text-muted-foreground">
        {{ tf('channelEditor.basic.serviceType.label', '上游类型') }}
      </span>
      <Select :model-value="serviceType" @update:model-value="(val) => emit('update:service-type', String(val))">
        <SelectTrigger class="h-9 w-[200px] bg-background">
          <SelectValue :placeholder="tf('channelEditor.basic.serviceType.placeholder', '选择类型')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="opt in serviceTypeOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <!-- 编辑模式：显示操作按钮 -->
    <div v-if="isEditMode" class="flex shrink-0 items-center gap-1.5">
      <Button
        variant="ghost"
        size="icon-sm"
        class="h-8 w-8 rounded-full text-muted-foreground transition-all hover:bg-primary/10 hover:text-primary"
        :title="noVision ? tf('channelEditor.compat.visionDisabled', '视觉已禁用') : tf('channelEditor.compat.visionEnabled', '视觉已启用')"
        @click="emit('toggle-no-vision')"
      >
        <EyeOff v-if="noVision" class="h-3.5 w-3.5 text-amber-500" />
        <Eye v-else class="h-3.5 w-3.5" />
      </Button>
      <Button
        v-if="channelType !== 'images'"
        variant="outline"
        size="sm"
        class="h-8 rounded-full border border-border/80 bg-background/50 px-3.5 shadow-sm hover:bg-accent"
        :disabled="saving"
        @click="emit('test-capability')"
      >
        <Zap class="mr-1 h-3.5 w-3.5 fill-amber-500/20 text-amber-500" />
        {{ tf('capability.startTest', '能力测试') }}
      </Button>
    </div>
  </div>
</template>
