// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import ProtocolModelAvailability from './ProtocolModelAvailability.vue'

vi.mock('../../i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, number>) => params?.count === undefined ? key : `${key}:${params.count}`,
  }),
}))

const passthroughStub = defineComponent({
  template: '<span><slot /></span>',
})

describe('ProtocolModelAvailability', () => {
  it('按协议分组展示各自的可用模型', () => {
    const wrapper = mount(ProtocolModelAvailability, {
      props: {
        routes: [
          {
            kind: 'messages', index: 0, name: 'fastaitoken-claude', serviceType: 'claude',
            supportedModels: ['gpt-5.6-terra', 'gpt-5.6-sol', 'gpt-5.6-sol'],
          },
          {
            kind: 'chat', index: 0, name: 'fastaitoken-chat', serviceType: 'openai',
            supportedModels: ['gpt-5.6-sol'],
          },
          {
            kind: 'responses', index: 0, name: 'fastaitoken-codex', serviceType: 'responses',
            supportedModels: ['codex-auto-review'],
          },
        ],
      },
      global: {
        stubs: {
          VChip: passthroughStub,
          VIcon: passthroughStub,
        },
      },
    })

    const messages = wrapper.get('[data-kind="messages"]')
    const chat = wrapper.get('[data-kind="chat"]')
    const responses = wrapper.get('[data-kind="responses"]')

    expect(messages.text()).toContain('/v1/messages')
    expect(messages.text()).toContain('gpt-5.6-sol')
    expect(messages.text()).toContain('gpt-5.6-terra')
    expect(messages.text().match(/gpt-5\.6-sol/g)).toHaveLength(1)
    expect(chat.text()).toContain('/v1/chat/completions')
    expect(chat.text()).not.toContain('gpt-5.6-terra')
    expect(responses.text()).toContain('/v1/responses')
    expect(responses.text()).toContain('codex-auto-review')
  })

  it('区分未记录模型范围与协议不可用', () => {
    const wrapper = mount(ProtocolModelAvailability, {
      props: {
        routes: [{ kind: 'gemini', index: 0, name: 'gemini', serviceType: 'gemini' }],
      },
      global: {
        stubs: {
          VChip: passthroughStub,
          VIcon: passthroughStub,
        },
      },
    })

    expect(wrapper.get('[data-kind="gemini"]').text()).toContain('channelEditor.protocolModels.empty')
  })
})
