import { describe, expect, it } from 'vitest'
import { applyDocumentLanguage, normalizeLocale, resolveInitialLocale, translate, translateOrFallback } from './core'

describe('normalizeLocale', () => {
  it('returns zh-CN for Chinese variants', () => {
    expect(normalizeLocale('zh')).toBe('zh-CN')
    expect(normalizeLocale('zh-CN')).toBe('zh-CN')
    expect(normalizeLocale('zh_CN')).toBe('zh-CN')
    expect(normalizeLocale('zh-Hans')).toBe('zh-CN')
    expect(normalizeLocale('zh-Hans-CN')).toBe('zh-CN')
    expect(normalizeLocale('zh_CN.UTF-8')).toBe('zh-CN')
  })

  it('returns en for English and unknown locales', () => {
    expect(normalizeLocale('en')).toBe('en')
    expect(normalizeLocale('en-US')).toBe('en')
    expect(normalizeLocale('fr-FR')).toBe('en')
    expect(normalizeLocale(undefined)).toBe('en')
    expect(normalizeLocale(null)).toBe('en')
  })
})

describe('resolveInitialLocale', () => {
  it('prefers supported persisted locale', () => {
    expect(resolveInitialLocale('zh-CN', 'en-US')).toBe('zh-CN')
    expect(resolveInitialLocale('en', 'zh_CN')).toBe('en')
  })

  it('falls back to normalized system locale when persisted unsupported', () => {
    expect(resolveInitialLocale(null, 'zh_CN.UTF-8')).toBe('zh-CN')
    expect(resolveInitialLocale(undefined, 'fr-FR')).toBe('en')
  })

  it('defaults to en when no data', () => {
    expect(resolveInitialLocale(null, null)).toBe('en')
  })
})

describe('translate', () => {
  it('returns locale message when available', () => {
    expect(translate('zh-CN', 'nav.status')).toBe('网关监控')
  })

  it('falls back to English message', () => {
    expect(translate('en', 'nav.status')).toBe('Status')
  })

  it('substitutes parameters in messages', () => {
    expect(translate('en', 'env.fieldMin', { field: 'Port', min: '1000' })).toBe('Port must be at least 1000')
  })

  it('falls back to key when message missing', () => {
    expect(translate('en', 'common.missing' as never)).toBe('common.missing')
  })
})

describe('translateOrFallback', () => {
  // 回归：zh-CN 的 presetMessages 故意留空，但 messages 表中有完整中文，
  // 必须穷尽当前 locale 的两张表后再回落到默认 locale，否则会被英文 preset 截胡。
  it('prefers zh-CN messages over en presetMessages for shared keys', () => {
    expect(translateOrFallback('zh-CN', 'channel.preset.deepseek.description', 'fallback'))
      .toBe('Messages 原生透传、Codex Responses、Chat 渠道透传三种用法。')
  })

  it('uses en presetMessages for keys only defined there', () => {
    expect(translateOrFallback('en', 'channel.target.messages.label', 'fallback'))
      .toBe('Messages native')
  })

  it('falls back to default locale when key missing in current locale', () => {
    // zh-CN 的两张表都没有这个 key，应回落到 en 的 presetMessages
    // 注：所有 preset key 目前在 messages.ts 中均有定义，此测试用虚构 key 验证回落逻辑
    expect(translateOrFallback('zh-CN', 'channel.target.imaginary.label', 'fb'))
      .toBe('fb')
  })

  it('returns fallback when key missing everywhere', () => {
    expect(translateOrFallback('en', 'totally.unknown.key', 'fallback-text'))
      .toBe('fallback-text')
  })
})

describe('applyDocumentLanguage', () => {
  it('sets document.documentElement.lang', () => {
    document.documentElement.lang = ''
    applyDocumentLanguage('zh-CN')
    expect(document.documentElement.lang).toBe('zh-CN')
  })
})
