import { useAdminApi } from '@/composables/useAdminApi'
import { filterOutKey, mergeAccount } from '@/composables/copilotKeyPool'
import { COPILOT_OAUTH_VERIFY_PATH, type Channel, type CopilotVerifyResponse } from '@/services/admin-api'

/**
 * GitHub Copilot 多账号「同渠道多 key 池」管理。
 *
 * 后端 UpdateUpstream 对 apiKeys/apiKeyConfigs 是整体替换（非合并），
 * 因此增删账号都必须「先读现有渠道 -> 本地合并/过滤 -> 整体回写」。
 * 纯合并逻辑见 copilotKeyPool.ts；此处只负责授权校验与 admin API 读写。
 */

// 重新导出纯函数，供组件按需引用。
export { buildBaseConfigs, filterOutKey, maskKey, mergeAccount, type MergedKeyPool } from '@/composables/copilotKeyPool'

export function useCopilotAccounts() {
  const adminApi = useAdminApi()

  /** 用 GitHub OAuth token 反查其 GitHub 用户名（login）。 */
  async function verifyAccount(token: string, proxyUrl?: string): Promise<string> {
    const resp = await adminApi.post<CopilotVerifyResponse>(COPILOT_OAUTH_VERIFY_PATH, {
      accessToken: token,
      proxyUrl: proxyUrl?.trim() || undefined,
    })
    return (resp.login || '').trim()
  }

  /** 把账号合并进渠道 key 池并整体回写。channel 必须含最新 index。 */
  async function addAccount(target: string, channel: Channel, token: string, login: string): Promise<void> {
    const pool = mergeAccount(channel, token, login)
    await adminApi.put(`/api/${target}/channels/${channel.index}`, pool)
  }

  /**
   * 从渠道 key 池移除某账号。
   * 移除最后一个账号时删除整个渠道；否则整体回写剩余 key 池。
   */
  async function removeAccount(target: string, channel: Channel, key: string): Promise<void> {
    const pool = filterOutKey(channel, key)
    if (pool.apiKeys.length === 0) {
      await adminApi.del(`/api/${target}/channels/${channel.index}`)
      return
    }
    await adminApi.put(`/api/${target}/channels/${channel.index}`, pool)
  }

  return { verifyAccount, addAccount, removeAccount }
}
