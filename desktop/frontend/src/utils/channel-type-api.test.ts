import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getChannelTypeApi } from './channel-type-api'

const adminApi = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  patch: vi.fn(),
  del: vi.fn(),
}))

vi.mock('@/composables/useAdminApi', () => ({
  useAdminApi: () => adminApi,
}))

describe('getChannelTypeApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sends apiKey when adding a channel key', async () => {
    await getChannelTypeApi('vectors').addApiKey(2, 'sk-test')

    expect(adminApi.post).toHaveBeenCalledWith('/api/vectors/channels/2/keys', {
      apiKey: 'sk-test',
    })
  })

  it('keeps api keys encoded in path operations', async () => {
    await getChannelTypeApi('vectors').removeApiKey(2, 'sk/a+b')
    await getChannelTypeApi('vectors').moveApiKeyToTop(2, 'sk/a+b')
    await getChannelTypeApi('vectors').moveApiKeyToBottom(2, 'sk/a+b')

    expect(adminApi.del).toHaveBeenCalledWith('/api/vectors/channels/2/keys/sk%2Fa%2Bb')
    expect(adminApi.post).toHaveBeenCalledWith('/api/vectors/channels/2/keys/sk%2Fa%2Bb/top')
    expect(adminApi.post).toHaveBeenCalledWith('/api/vectors/channels/2/keys/sk%2Fa%2Bb/bottom')
  })
})
