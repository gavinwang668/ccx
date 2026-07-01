import { describe, expect, it } from 'vitest'
import { buildBaseConfigs, filterOutKey, maskKey, mergeAccount } from '@/composables/copilotKeyPool'
import type { Channel } from '@/services/admin-api'

// 构造最小 channel 存根（仅用到 apiKeys/apiKeyConfigs）。
function channel(apiKeys: string[], apiKeyConfigs?: Channel['apiKeyConfigs']): Pick<Channel, 'apiKeys' | 'apiKeyConfigs'> {
  return { apiKeys, apiKeyConfigs }
}

describe('mergeAccount', () => {
  it('空渠道加入首个账号', () => {
    const pool = mergeAccount(channel([]), 'tok-A', 'alice')
    expect(pool.apiKeys).toEqual(['tok-A'])
    expect(pool.apiKeyConfigs).toEqual([{ key: 'tok-A', name: 'alice' }])
  })

  it('渠道已有无名 key（首次建渠道）时补 name 而非重复', () => {
    const pool = mergeAccount(channel(['tok-A']), 'tok-A', 'alice')
    expect(pool.apiKeys).toEqual(['tok-A'])
    expect(pool.apiKeyConfigs).toEqual([{ key: 'tok-A', name: 'alice' }])
  })

  it('加入第二个账号', () => {
    const pool = mergeAccount(channel(['tok-A'], [{ key: 'tok-A', name: 'alice' }]), 'tok-B', 'bob')
    expect(pool.apiKeys).toEqual(['tok-A', 'tok-B'])
    expect(pool.apiKeyConfigs).toEqual([
      { key: 'tok-A', name: 'alice' },
      { key: 'tok-B', name: 'bob' },
    ])
  })

  it('重复授权同账号则更新其 token，不新增', () => {
    const pool = mergeAccount(channel(['tok-A'], [{ key: 'tok-A', name: 'alice' }]), 'tok-A2', 'alice')
    expect(pool.apiKeys).toEqual(['tok-A2'])
    expect(pool.apiKeyConfigs).toEqual([{ key: 'tok-A2', name: 'alice' }])
  })

  it('保留已有 config 的其他字段', () => {
    const pool = mergeAccount(
      channel(['tok-A'], [{ key: 'tok-A', name: 'alice', weight: 5 }]),
      'tok-A2',
      'alice',
    )
    expect(pool.apiKeyConfigs).toEqual([{ key: 'tok-A2', name: 'alice', weight: 5 }])
  })
})

describe('filterOutKey', () => {
  it('移除一个账号保留其余', () => {
    const pool = filterOutKey(
      channel(['tok-A', 'tok-B'], [{ key: 'tok-A', name: 'alice' }, { key: 'tok-B', name: 'bob' }]),
      'tok-B',
    )
    expect(pool.apiKeys).toEqual(['tok-A'])
    expect(pool.apiKeyConfigs).toEqual([{ key: 'tok-A', name: 'alice' }])
  })

  it('移除最后一个账号得到空池', () => {
    const pool = filterOutKey(channel(['tok-A'], [{ key: 'tok-A', name: 'alice' }]), 'tok-A')
    expect(pool.apiKeys).toEqual([])
    expect(pool.apiKeyConfigs).toEqual([])
  })
})

describe('buildBaseConfigs', () => {
  it('apiKeys 为权威列表，缺失 config 补空', () => {
    const base = buildBaseConfigs(channel(['tok-A', 'tok-B'], [{ key: 'tok-A', name: 'alice' }]))
    expect(base).toEqual([{ key: 'tok-A', name: 'alice' }, { key: 'tok-B' }])
  })
})

describe('maskKey', () => {
  it('长 key 首尾各留 4 位', () => {
    expect(maskKey('gho_1234567890abcdef')).toBe('gho_••••cdef')
  })
  it('过短 key 整体掩码', () => {
    expect(maskKey('short')).toBe('••••')
  })
})
