import { http } from './http'
import type { Video } from './types'

export function likeAction(video_id: number) {
  return http.post<{ message: string }>('/api/v1/like/action', { video_id })
}
export function unlike(video_id: number) {
  return http.post<{ message: string }>('/api/v1/like/unlike', { video_id })
}
export function isLiked(video_id: number) {
  return http.post<{ is_liked: boolean }>('/api/v1/like/is_liked', { video_id })
}
export function myLikedVideos() {
  return http.post<Video[]>('/api/v1/like/my_videos', {})
}
