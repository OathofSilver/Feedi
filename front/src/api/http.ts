// 轻量 HTTP 封装：自动带 access token，401 时用 refresh_token 换新并重放一次。
// 统一把 {error} 抛出为带 message 的 Error（后端很多业务错误走 500，须透传文案）。

let accessToken = ''
let refreshToken = ''
// 本地缓存 token 的 key
const AT = 'feed_at'
const RT = 'feed_rt'

export function loadTokens() {
  try {
    accessToken = localStorage.getItem(AT) || ''
    refreshToken = localStorage.getItem(RT) || ''
  } catch {
    accessToken = ''
    refreshToken = ''
  }
}

export function setTokens(at: string, rt: string) {
  accessToken = at
  refreshToken = rt
  try {
    localStorage.setItem(AT, at)
    if (rt) localStorage.setItem(RT, rt)
  } catch {
    /* ignore */
  }
}

export function clearTokens() {
  accessToken = ''
  refreshToken = ''
  try {
    localStorage.removeItem(AT)
    localStorage.removeItem(RT)
  } catch {
    /* ignore */
  }
}

export function hasToken() {
  return !!accessToken
}

let refreshing: Promise<boolean> | null = null

async function doRefresh(): Promise<boolean> {
  if (!refreshToken) return false
  try {
    const res = await rawPost<{ token: string; refresh_token?: string }>(
      '/api/v1/account/refresh',
      { refresh_token: refreshToken },
      false,
    )
    accessToken = res.token
    try {
      localStorage.setItem(AT, res.token)
    } catch {
      /* ignore */
    }
    return true
  } catch {
    clearTokens()
    return false
  }
}

/** 供模块调用：确保拿到有效 token */
export async function ensureToken(): Promise<boolean> {
  if (accessToken) return true
  if (refreshing) return refreshing
  refreshing = doRefresh().finally(() => {
    refreshing = null
  })
  return refreshing
}

async function rawPost<T>(url: string, body: unknown, withToken: boolean): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (withToken && accessToken) headers['Authorization'] = `Bearer ${accessToken}`

  const resp = await fetch(url, {
    method: 'POST',
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) {
    const msg = (data as { error?: string }).error || `请求失败(${resp.status})`
    const err = new Error(msg) as Error & { status?: number }
    err.status = resp.status
    throw err
  }
  return data as T
}

/**
 * 核心请求：带 token 的 POST。若遇 401，先刷新 token 重放一次。
 * @param retried 内部标记，避免递归死循环
 */
async function post<T>(url: string, body?: unknown, retried = false): Promise<T> {
  // 未登录时若需要鉴权则直接尝试（多数接口游客可访问）
  if (!accessToken) await loadTokens()
  try {
    return await rawPost<T>(url, body, true)
  } catch (e) {
    const status = (e as { status?: number }).status
    if (status === 401 && !retried) {
      const ok = await doRefresh()
      if (ok) return rawPost<T>(url, body, true)
    }
    throw e
  }
}

/** 游客可访问接口（不强制带 token，但带上能拿到个性化状态） */
async function postPublic<T>(url: string, body?: unknown): Promise<T> {
  // 尝试带 token 发，失败（401/无token）则匿名重发
  if (!accessToken) await loadTokens()
  if (!accessToken) return rawPost<T>(url, body, false)
  try {
    return await rawPost<T>(url, body, true)
  } catch (e) {
    const status = (e as { status?: number }).status
    if (status === 401) {
      const ok = await doRefresh()
      if (ok) return rawPost<T>(url, body, true)
      return rawPost<T>(url, body, false)
    }
    throw e
  }
}

/** multipart 上传：带 token */
async function upload<T>(url: string, form: FormData): Promise<T> {
  if (!accessToken) await loadTokens()
  // 不手动设置 Content-Type，交由浏览器补 boundary
  let headers: Record<string, string> = {}
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`
  const resp = await fetch(url, { method: 'POST', headers, body: form })
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) {
    const msg = (data as { error?: string }).error || `上传失败(${resp.status})`
    const err = new Error(msg) as Error & { status?: number }
    err.status = resp.status
    throw err
  }
  return data as T
}

export const http = {
  post,
  postPublic,
  upload,
  get accessToken() {
    return accessToken
  },
}

export async function sseConnect(
  url: string,
  onData: (raw: string) => void,
  onError: () => void,
): Promise<() => void> {
  if (!accessToken) await loadTokens()
  const controller = new AbortController()
  try {
    const resp = await fetch(url, {
      headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
      signal: controller.signal,
    })
    const reader = resp.body?.getReader()
    if (!reader) throw new Error('no body')

    const decoder = new TextDecoder()
    let buf = ''
    const pump = async () => {
      try {
        for (;;) {
          const { done, value } = await reader.read()
          if (done) break
          buf += decoder.decode(value, { stream: true })
          let idx: number
          while ((idx = buf.indexOf('\n\n')) >= 0) {
            const chunk = buf.slice(0, idx)
            buf = buf.slice(idx + 2)
            // 只取 data: 行
            const dataLine = chunk
              .split('\n')
              .find((l) => l.startsWith('data:'))
              ?.slice(5)
              .trim()
            if (dataLine) onData(dataLine)
          }
        }
      } catch {
        /* aborted or err */
      }
      if (!controller.signal.aborted) onError()
    }
    pump()
  } catch {
    onError()
  }
  return () => controller.abort()
}
