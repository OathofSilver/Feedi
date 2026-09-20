import { http } from './http'
import type { FollowerItem } from './types'

export function follow(vlogger_id: number) {
  return http.post<{ message: string }>('/api/v1/social/follow', { vlogger_id })
}
export function unfollow(vlogger_id: number) {
  return http.post<{ message: string }>('/api/v1/social/unfollow', { vlogger_id })
}

/**
 * 查询某用户的粉丝列表。
 * 后端允许显式传 vlogger_id 查询任意用户；不传(=0)时默认返回当前登录者。
 */
export function myFollowers(vlogger_id = 0) {
  return http.post<{ followers: FollowerItem[]; follower_count: number }>('/api/v1/social/followers', {
    ...(vlogger_id ? { vlogger_id } : {}),
  })
}

/**
 * 查询某用户关注的人（vloggers）。
 * 后端允许显式传 follower_id 查询任意用户；不传(=0)时默认返回当前登录者。
 */
export function myVloggers(follower_id = 0) {
  return http.post<{ vloggers: FollowerItem[]; vlogger_count: number }>('/api/v1/social/vloggers', {
    ...(follower_id ? { follower_id } : {}),
  })
}

/**
 * 社交计数。可传 user_id 查询任意用户；不传(=0)时默认返回当前登录者。
 */
export function socialCounts(user_id = 0) {
  return http.post<{ follower_count: number; vlogger_count: number }>('/api/v1/social/counts', {
    ...(user_id ? { user_id } : {}),
  })
}

/** 当前登录者是否已关注某用户 */
export function isFollowing(vlogger_id: number) {
  return http.post<{ is_following: boolean }>('/api/v1/social/is_following', { vlogger_id })
}
