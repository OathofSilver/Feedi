import { http, sseConnect } from './http'
import type { NotifPage } from './types'

export function listNotifications(before_time = 0, limit = 30) {
  return http.post<NotifPage>('/api/v1/notification/list', { before_time, limit })
}

export interface SseEvent {
  event_id?: string
  notification_id?: number
  recipient_id?: number
}

export function connectStream(onEvent: (e: SseEvent) => void, onClose: () => void) {
  return sseConnect('/api/v1/notification/stream', (raw) => {
    try {
      onEvent(JSON.parse(raw) as SseEvent)
    } catch {
      /* ignore malformed */
    }
  }, onClose)
}
