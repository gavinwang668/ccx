import { describe, expect, it } from 'vitest'
import { buildExpectedRequestUrls } from './expected-request-urls'

describe('buildExpectedRequestUrls', () => {
  it('builds the OpenAI embeddings endpoint for vectors channels', () => {
    const result = buildExpectedRequestUrls('vectors', 'openai', 'https://api.openai.com')

    expect(result).toEqual([
      {
        baseUrl: 'https://api.openai.com',
        expectedUrl: 'https://api.openai.com/v1/embeddings',
      },
    ])
  })

  it('keeps no-version semantics for vectors channels with #', () => {
    const result = buildExpectedRequestUrls('vectors', 'openai', 'https://api.example.com#')

    expect(result[0]?.expectedUrl).toBe('https://api.example.com/embeddings')
  })
})
