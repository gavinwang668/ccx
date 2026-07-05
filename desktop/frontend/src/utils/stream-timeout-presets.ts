export interface StreamTimeoutPreset {
  firstContentMs: number
  inactivityMs: number
  toolCallIdleMs: number
}

// 全局与渠道级共用的流式超时预设（三套固定值）
// 温和策略作为新默认：大幅延长超时窗口，降低误熔断概率
export const streamTimeoutPresets: Record<'gentle' | 'balanced' | 'aggressive', StreamTimeoutPreset> = {
  gentle: { firstContentMs: 240000, inactivityMs: 180000, toolCallIdleMs: 300000 },
  balanced: { firstContentMs: 90000, inactivityMs: 90000, toolCallIdleMs: 300000 },
  aggressive: { firstContentMs: 60000, inactivityMs: 60000, toolCallIdleMs: 180000 },
}

export const defaultStreamTimeouts = streamTimeoutPresets.balanced
