import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as accountApi from '@/api/account'
import { clearTokens, hasToken, loadTokens, setTokens } from '@/api/http'
import type { Account } from '@/api/types'

export const useAuthStore = defineStore('auth', () => {
  loadTokens()
  const isAuthed = ref(hasToken())
  // 仅缓存当前会话基础的账号信息
  const accountId = ref<number | null>(null)
  const username = ref('')
  const profile = ref<Account | null>(null)

  async function doLogin(u: string, p: string) {
    const res = await accountApi.login({ username: u, password: p })
    setTokens(res.token, res.refresh_token)
    isAuthed.value = true
    accountId.value = res.account_id
    username.value = res.username
    profile.value = {
      id: res.account_id,
      username: res.username,
    }
    // 登录后拉取完整资料（含头像），失败不影响登录主流程
    await refreshProfile()
    return res
  }

  async function doRegister(u: string, p: string) {
    await accountApi.register({ username: u, password: p })
    return doLogin(u, p)
  }

  /** 拉取当前账号完整资料到 profile（用于顶栏头像等展示） */
  async function refreshProfile() {
    if (!accountId.value) return
    try {
      const p = await accountApi.accountInfo(accountId.value)
      profile.value = p
      username.value = p.username || username.value
    } catch {
      /* 网络异常静默，保留已有字段 */
    }
  }

  /** 头像更新后同步到会话 profile（上传完成回写） */
  function setProfileAvatar(url: string) {
    if (profile.value) profile.value.avatar_url = url
  }

  function doLogout() {
    // 静默登出（后端可能失败也可忽略）
    accountApi.logout().catch(() => {
      /* ignore */
    })
    clearTokens()
    isAuthed.value = false
    accountId.value = null
    username.value = ''
    profile.value = null
  }

  function setUsername(u: string) {
    username.value = u
    if (profile.value) profile.value.username = u
  }

  return {
    isAuthed,
    accountId,
    username,
    profile,
    doLogin,
    doRegister,
    doLogout,
    setUsername,
    refreshProfile,
    setProfileAvatar,
  }
})
