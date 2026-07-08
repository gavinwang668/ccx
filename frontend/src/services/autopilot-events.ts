import { useAuthStore } from '@/stores/auth'
import { API_BASE } from './api-helpers'
import type { ProfileChangeEvent, ProfileChangelogResponse } from './api-types'

// ─── 辅助方法 ───

function getAuthHeaders(): Record<string, string> {
  const authStore = useAuthStore()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  const apiKey = authStore.apiKey as unknown as string | null
  if (apiKey) {
    headers['x-api-key'] = apiKey
  }
  return headers
}

/**
 * 将 API_BASE 转换为对应的 WebSocket URL。
 * - 生产环境 API_BASE 通常是相对路径（如 "/api"），需要拼接当前页面的 origin。
 * - 开发环境 API_BASE 可能是绝对地址（如 "http://localhost:3000/api"），
 *   只需替换协议前缀 http(s) -> ws(s)。
 */
function buildWsUrl(path: string): string {
  if (/^https?:\/\//i.test(API_BASE)) {
    return API_BASE.replace(/^http/i, 'ws') + path
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}${API_BASE}${path}`
}

// ─── API 方法 ───

/**
 * 拉取画像变更历史（REST）
 * GET /api/health-center/changelog
 */
export async function fetchProfileChangelog(params?: {
  channelUid?: string
  limit?: number
}): Promise<ProfileChangelogResponse> {
  const query = new URLSearchParams()
  if (params?.channelUid) query.set('channelUid', params.channelUid)
  if (params?.limit) query.set('limit', String(params.limit))
  const qs = query.toString()

  const url = `${API_BASE}/health-center/changelog${qs ? `?${qs}` : ''}`
  const response = await fetch(url, {
    method: 'GET',
    headers: getAuthHeaders(),
  })

  if (!response.ok) {
    const text = await response.text().catch(() => response.statusText)
    throw new Error(`fetch changelog failed (${response.status}): ${text}`)
  }

  return response.json()
}

export type ProfileEventsConnectionStatus = 'connecting' | 'open' | 'closed'

export interface ConnectProfileEventsOptions {
  onEvent: (event: ProfileChangeEvent) => void
  onStatusChange?: (status: ProfileEventsConnectionStatus) => void
}

/**
 * 建立画像变更事件 WebSocket 连接（实时推送）。
 * 浏览器原生 WebSocket 无法设置自定义请求头，鉴权 key 通过
 * Sec-WebSocket-Protocol 子协议传入（后端 middleware.getAPIKey 已支持回退读取）。
 * 断线自动重连（指数退避，1s 起步，封顶 30s），返回的 close() 用于组件卸载时清理，
 * 调用 close() 后不再重连。
 */
export function connectProfileEvents(options: ConnectProfileEventsOptions): () => void {
  const authStore = useAuthStore()
  let closedByCaller = false
  let socket: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let backoffMs = 1000
  const maxBackoffMs = 30000

  const notifyStatus = (status: ProfileEventsConnectionStatus) => {
    options.onStatusChange?.(status)
  }

  const connect = () => {
    if (closedByCaller) return

    const apiKey = authStore.apiKey as unknown as string | null
    const url = buildWsUrl('/health-center/events')

    notifyStatus('connecting')
    socket = apiKey ? new WebSocket(url, [apiKey]) : new WebSocket(url)

    socket.onopen = () => {
      backoffMs = 1000 // 连接成功后重置退避
      notifyStatus('open')
    }

    socket.onmessage = (event: MessageEvent<string>) => {
      try {
        const parsed = JSON.parse(event.data) as ProfileChangeEvent
        options.onEvent(parsed)
      } catch {
        // 忽略无法解析的消息，不影响连接
      }
    }

    socket.onclose = () => {
      notifyStatus('closed')
      if (closedByCaller) return
      reconnectTimer = setTimeout(connect, backoffMs)
      backoffMs = Math.min(backoffMs * 2, maxBackoffMs)
    }

    socket.onerror = () => {
      // onclose 会随后触发，重连逻辑统一在那里处理
    }
  }

  connect()

  return () => {
    closedByCaller = true
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (socket) {
      socket.onopen = null
      socket.onmessage = null
      socket.onclose = null
      socket.onerror = null
      socket.close()
      socket = null
    }
  }
}
