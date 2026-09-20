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

/** 小文件直传（兼容保留；视频主流程已走分片 uploadVideoChunked） */
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

// ============ 分片上传（后端 internal/chunk） ============

/** 单分片大小：与后端 chunk.ChunkSize (5MB) 保持一致 */
export const CHUNK_SIZE = 5 * 1024 * 1024
/** 分片上传并发数 */
const CHUNK_CONCURRENCY = 3

async function sha256Hex(buf: ArrayBuffer): Promise<string> {
  // crypto.subtle 仅在安全上下文可用（https / localhost）
  if (!crypto?.subtle) {
    throw new Error('当前环境不支持分片上传（需 HTTPS 安全上下文）')
  }
  const dg = await crypto.subtle.digest('SHA-256', buf)
  return Array.from(new Uint8Array(dg))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

export interface ChunkUploadResp {
  url: string
  play_url: string
}

/** 上传单分片（multipart: upload_id/chunk_index/chunk_hash/file） */
function uploadVideoChunk(uploadId: string, chunkIndex: number, chunkHash: string, blob: Blob) {
  const fd = new FormData()
  fd.append('upload_id', uploadId)
  fd.append('chunk_index', String(chunkIndex))
  fd.append('chunk_hash', chunkHash)
  fd.append('file', blob, 'chunk')
  return http.upload<{ chunk_index: number }>('/api/v1/video/upload/chunk', fd)
}

/**
 * 视频分片上传：init -> 逐片（并发3，单片失败重试2次）-> complete。
 * init 按「账号 + 全文件 hash」找回历史会话，实现断点续传（已传分片自动跳过）。
 * @param onProgress 回调已上传片数/总片数（uploaded 不含跳过片时以总待传片计）
 */
export async function uploadVideoChunked(
  file: File,
  onProgress?: (uploaded: number, pendingTotal: number) => void,
): Promise<ChunkUploadResp> {
  const totalChunks = Math.max(1, Math.ceil(file.size / CHUNK_SIZE))

  // 全文件 SHA-256：同账号重复上传同一文件时由后端定位既有会话
  const fileHash = await sha256Hex(await file.arrayBuffer())

  const initRes = await http.post<{ upload_id: string; uploaded_chunks: number[] }>(
    '/api/v1/video/upload/init',
    {
      filename: file.name,
      file_size: file.size,
      chunk_size: CHUNK_SIZE,
      total_chunks: totalChunks,
      file_hash: fileHash,
    },
  )

  const doneSet = new Set(initRes.uploaded_chunks || [])
  const pending: number[] = []
  for (let i = 0; i < totalChunks; i++) {
    if (!doneSet.has(i)) pending.push(i)
  }
  onProgress?.(0, pending.length)

  if (pending.length === 0) return complete()

  // 小并发 worker 池逐片上传
  let cursor = 0
  let sent = 0
  async function worker() {
    while (cursor < pending.length) {
      const idx = pending[cursor++]
      const start = idx * CHUNK_SIZE
      const blob = file.slice(start, Math.min(file.size, start + CHUNK_SIZE))
      const chunkHash = await sha256Hex(await blob.arrayBuffer())
      let attempt = 0
      for (;;) {
        try {
          await uploadVideoChunk(initRes.upload_id, idx, chunkHash, blob)
          break
        } catch (e) {
          if (++attempt >= 3) throw e
          await new Promise((r) => setTimeout(r, 400 * attempt))
        }
      }
      sent++
      onProgress?.(sent, pending.length)
    }
  }
  await Promise.all(
    Array.from({ length: Math.min(CHUNK_CONCURRENCY, pending.length) }, () => worker()),
  )

  return complete()

  async function complete() {
    return http.post<ChunkUploadResp>('/api/v1/video/upload/complete', {
      upload_id: initRes.upload_id,
    })
  }
}
