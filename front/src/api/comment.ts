import { http } from './http'
import type { CommentItem } from './types'

export function listComments(video_id: number) {
  return http.postPublic<CommentItem[]>('/api/v1/comment/list', { video_id })
}
export function publishComment(video_id: number, content: string) {
  return http.post<{ message: string }>('/api/v1/comment/publish', { video_id, content })
}
export function deleteComment(comment_id: number) {
  return http.post<{ message: string }>('/api/v1/comment/delete', { comment_id })
}
