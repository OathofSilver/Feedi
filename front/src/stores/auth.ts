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
    return res
  }

  async function doRegister(u: string, p: string) {
    await accountApi.register({ username: u, password: p })
    return doLogin(u, p)
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

  return { isAuthed, accountId, username, profile, doLogin, doRegister, doLogout, setUsername }
})
