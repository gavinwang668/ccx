import type { APIKeyConfig, Channel } from '@/services/admin-api'

/**
 * GitHub Copilot 多账号「同渠道多 key 池」的纯逻辑。
 * 无副作用、不依赖任何 composable/binding，便于单元测试。
 *
 * 账号唯一键取 GitHub login（存入 APIKeyConfig.name）；
 * 由 apiKeyConfigs 派生 apiKeys，保证两数组顺序对齐。
 */

export interface MergedKeyPool {
  apiKeys: string[]
  apiKeyConfigs: APIKeyConfig[]
}

type PoolSource = Pick<Channel, 'apiKeys' | 'apiKeyConfigs'>

/**
 * 以渠道现有 apiKeys 为权威列表构造 config 基线，
 * 保留每个 key 已有的 config（含 name 及其他字段），缺失的补空 config。
 */
export function buildBaseConfigs(channel: PoolSource): APIKeyConfig[] {
  const keys = channel.apiKeys ?? []
  const byKey = new Map<string, APIKeyConfig>((channel.apiKeyConfigs ?? []).map(c => [c.key, c]))
  return keys.map(key => byKey.get(key) ?? { key })
}

/** 按 key 去重（保留首次出现），并由 configs 派生 apiKeys。 */
function toPool(configs: APIKeyConfig[]): MergedKeyPool {
  const seen = new Set<string>()
  const deduped = configs.filter((c) => {
    if (!c.key || seen.has(c.key)) return false
    seen.add(c.key)
    return true
  })
  return { apiKeys: deduped.map(c => c.key), apiKeyConfigs: deduped }
}

/**
 * 合并一个账号（token + login）进 key 池。
 * - 已存在同 login 的项 -> 更新其 token（token 轮换）
 * - 否则若已存在同 token 的项 -> 补齐其 name（如首次建渠道后补 name）
 * - 都没有 -> 追加新项
 */
export function mergeAccount(channel: PoolSource, token: string, login: string): MergedKeyPool {
  const base = buildBaseConfigs(channel)
  let idx = base.findIndex(c => c.name && c.name === login)
  if (idx < 0) idx = base.findIndex(c => c.key === token)
  const merged = idx >= 0
    ? base.map((c, i) => (i === idx ? { ...c, key: token, name: login } : c))
    : [...base, { key: token, name: login }]
  return toPool(merged)
}

/** 从 key 池移除指定 key，返回过滤后的 apiKeys/apiKeyConfigs。 */
export function filterOutKey(channel: PoolSource, key: string): MergedKeyPool {
  return toPool(buildBaseConfigs(channel).filter(c => c.key !== key))
}

/** key 掩码展示：首尾各保留 4 位，过短则整体掩码。 */
export function maskKey(key: string): string {
  const value = (key ?? '').trim()
  if (value.length <= 8) return '••••'
  return `${value.slice(0, 4)}••••${value.slice(-4)}`
}
