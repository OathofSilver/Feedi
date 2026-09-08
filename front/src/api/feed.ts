import { http } from './http'
import type { FeedPage } from './types'

/** 最新视频 —— latest_time 为毫秒 */
export interface LatestReq {
  limit?: number
  latest_time?: number
}
export function feedLatest(p: LatestReq = {}) {
  return http.postPublic<FeedPage>('/api/v1/feed/latest', {
    limit: p.limit ?? 10,
    latest_time: p.latest_time ?? 0,
  })
}

/** 关注流 —— latest_time 为秒 */
export function feedFollowing(p: { limit?: number; latest_time?: number } = {}) {
  return http.post<FeedPage>('/api/v1/feed/following', {
    limit: p.limit ?? 10,
    latest_time: p.latest_time ?? 0,
  })
}

/** 点赞热门榜 —— 成对游标 */
export function feedLikesCount(p: {
  limit?: number
  likes_count_before?: number
  id_before?: number
} = {}) {
  const body: Record<string, number> = { limit: p.limit ?? 10 }
  if (p.likes_count_before !== undefined && p.id_before !== undefined) {
    body.likes_count_before = p.likes_count_before
    body.id_before = p.id_before
  }
  return http.postPublic<FeedPage>('/api/v1/feed/likes_count', body)
}

/** 热度榜 —— 传 offset */
export function feedPopularity(p: { limit?: number; as_of?: number; offset?: number } = {}) {
  return http.postPublic<FeedPage>('/api/v1/feed/popularity', {
    limit: p.limit ?? 10,
    as_of: p.as_of ?? 0,
    offset: p.offset ?? 0,
    latest_popularity: 0,
    latest_before: '0001-01-01T00:00:00Z',
    latest_id_before: 0,
  })
}

/** 按标签 */
export function feedTag(tag_name: string, limit = 20) {
  return http.postPublic<FeedPage>('/api/v1/feed/tag', { tag_name, limit })
}
