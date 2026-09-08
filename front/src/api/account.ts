import { http } from './http'
import type { Account, LoginResp } from './types'

export interface AuthPayload {
  username: string
  password: string
}

export function register(payload: AuthPayload) {
  return http.post<{ message: string }>('/api/v1/account/register', payload)
}

export function login(payload: AuthPayload) {
  return http.post<LoginResp>('/api/v1/account/login', payload)
}

export function accountInfo(id: number) {
  return http.postPublic<Account>('/api/v1/account/info', { id })
}

export function findByName(username: string) {
  return http.postPublic<{ id: number; username: string }>('/api/v1/account/username', { username })
}

export function logout() {
  return http.post<{ message: string }>('/api/v1/account/logout', {})
}

export function rename(new_username: string) {
  return http.post<{ token: string }>('/api/v1/account/rename', { new_username })
}

export function updateProfile(payload: { avatar_url?: string; bio?: string }) {
  return http.post<{ message: string }>('/api/v1/account/profile', payload)
}

export function uploadAvatar(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return http.upload<{ url: string; message: string }>('/api/v1/account/avatar', fd)
}
