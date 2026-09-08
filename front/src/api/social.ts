import { http } from './http'
import type { FollowerItem } from './types'

export function follow(vlogger_id: number) {
  return http.post<{ message: string }>('/api/v1/social/follow', { vlogger_id })
}
export function unfollow(vlogger_id: number) {
  return http.post<{ message: string }>('/api/v1/social/unfollow', { vlogger_id })
}
export function myFollowers() {
  return http.post<{ followers: FollowerItem[]; follower_count: number }>('/api/v1/social/followers', {})
}
export function myVloggers() {
  return http.post<{ vloggers: FollowerItem[]; vlogger_count: number }>('/api/v1/social/vloggers', {})
}
export function socialCounts() {
  return http.post<{ follower_count: number; vlogger_count: number }>('/api/v1/social/counts', {})
}
