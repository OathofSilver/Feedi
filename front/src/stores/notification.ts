import { defineStore } from 'pinia'
import { ref, computed, watch, onScopeDispose } from 'vue'
import { listNotifications, connectStream } from '@/api/notification'
import { useAuthStore } from './auth'
import type { NotificationItem } from '@/api/types'

/**
 * 通知模块：列表拉取 + SSE 实时推送 + 未读红点。
 * 生命周期与登录态强绑定：登录后自动开 SSE，登出自动断开。
 * 组件只需把「红点/列表/开关」接到本 store 即可，无需关心连接细节。
 */
export const useNotificationStore = defineStore('notification', () => {
  const auth = useAuthStore()
  const notifList = ref<NotificationItem[]>([])
  const unread = ref(0)
  const panelOpen = ref(false)

  let cleanup: (() => void) | null = null
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let lastSeenId = 0
  let fetching = false

  const unreadCount = computed(() => unread.value)

  /** 拉取最新一页列表并刷新未读计数 */
  async function refresh(silent = true) {
    if (!auth.isAuthed || fetching) return
    fetching = true
    try {
      const res = await listNotifications(0, 40)
      const newestId = res.notifications[0]?.id ?? 0
      notifList.value = res.notifications
      if (!panelOpen.value && newestId > lastSeenId && lastSeenId !== 0) {
        unread.value = 1
      }
    } catch {
      /* 后端未就绪/网络异常：静默，避免打扰 */
    } finally {
      fetching = false
    }
  }

  /** 打开消息抽屉/通知页：清未读并记录已读水位 */
  function markSeen() {
    panelOpen.value = true
    unread.value = 0
    if (notifList.value[0]) lastSeenId = notifList.value[0].id
  }
  function closePanel() {
    panelOpen.value = false
  }

  /** 建立 SSE 连接（登录后调用） */
  function open() {
    if (!auth.isAuthed) return
    cleanup?.()
    refresh()
    cleanup = connectStream(
      // 收到新通知事件：提示 + 刷新列表
      () => {
        if (!panelOpen.value) unread.value = 1
        refresh()
      },
      // 断线重连
      () => {
        if (auth.isAuthed && retryTimer == null) {
          retryTimer = setTimeout(() => {
            retryTimer = null
            open()
          }, 4000)
        }
      },
    )
  }

  /** 断开连接（登出/卸载时） */
  function close() {
    cleanup?.()
    cleanup = null
    if (retryTimer) {
      clearTimeout(retryTimer)
      retryTimer = null
    }
  }

  /** 全量拉取列表（主动打开通知页时用） */
  async function ensureList() {
    await refresh(false)
  }

  // 核心：跟随登录态自动开/关连接，并首次登录时拉列表。
  // 使模块与「在哪个页面挂载」解耦——任何页面登录后都能收到实时通知。
  watch(
    () => auth.isAuthed,
    (authed) => {
      if (authed) {
        lastSeenId = 0
        open()
      } else {
        close()
        notifList.value = []
        unread.value = 0
      }
    },
    { immediate: true },
  )

  onScopeDispose(close)

  return { notifList, unreadCount, refresh, ensureList, markSeen, closePanel, open, close }
})
