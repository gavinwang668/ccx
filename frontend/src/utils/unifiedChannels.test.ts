import { describe, expect, it } from 'vitest'

import type { Channel, ChannelsResponse } from '@/services/api'
import { buildUnifiedChannelsData, type LlmChannelKind } from './unifiedChannels'

const channel = (name: string, accountUid: string, index: number, apiKeys: string[]): Channel => ({
  name,
  accountUid,
  channelUid: `ch-${index}`,
  providerId: 'mimo',
  autoManaged: true,
  index,
  serviceType: name.endsWith('-claude') ? 'claude' : 'openai',
  baseUrl: 'https://example.com',
  apiKeys,
})

const response = (channels: Channel[]): ChannelsResponse => ({ channels, current: -1 })

describe('buildUnifiedChannelsData account grouping', () => {
  it('优先按 accountUid 聚合多协议渠道，不依赖 Key 指纹', () => {
    const data: Record<LlmChannelKind, ChannelsResponse> = {
      messages: response([channel('mimo-main-claude', 'acct-main', 0, ['sk-a'])]),
      chat: response([channel('mimo-main-chat', 'acct-main', 1, ['sk-a', 'sk-b'])]),
      responses: response([channel('mimo-main-codex', 'acct-main', 2, ['sk-b'])]),
      gemini: response([channel('mimo-main-gemini', 'acct-main', 3, ['sk-a'])]),
    }

    const result = buildUnifiedChannelsData(data)
    expect(result.channels).toHaveLength(1)
    expect(result.channels[0].accountUid).toBe('acct-main')
    expect(result.channels[0].protocolCapsules?.map(item => item.kind)).toEqual([
      'messages',
      'chat',
      'responses',
      'gemini',
    ])
  })

  it('相同 provider 和名称下不同 accountUid 不应合并', () => {
    const data: Record<LlmChannelKind, ChannelsResponse> = {
      messages: response([
        channel('mimo-main-claude', 'acct-a', 0, ['sk-a']),
        channel('mimo-main-claude', 'acct-b', 1, ['sk-b']),
      ]),
      chat: response([]),
      responses: response([]),
      gemini: response([]),
    }

    expect(buildUnifiedChannelsData(data).channels).toHaveLength(2)
  })
})
