import { http } from './http'
import type { Video } from './types'

export function videoDetail(id: number) {
  return http.postPublic<Video>('/api/v1/video/detail', { id })
}
export function publishVideo(payload: {
  title: string
  description: string
  play_url: string
  cover_url: string
}) {
  return http.post<Video>('/api/v1/video/publish', payload)
}
export function uploadVideoFile(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return http.upload<{ url: string; play_url: string }>('/api/v1/video/upload', fd)
}
export function uploadCoverFile(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return http.upload<{ url: string; cover_url: string }>('/api/v1/video/cover', fd)
}
export function deleteVideo(id: number) {
  return http.post<{ message: string }>('/api/v1/video/delete', { id })
}
