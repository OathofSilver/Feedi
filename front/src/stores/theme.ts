import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useThemeStore = defineStore('theme', () => {
  const KEY = 'feed_theme'
  const stored = (() => {
    try {
      return localStorage.getItem(KEY)
    } catch {
      return null
    }
  })()

  const theme = ref<'dark' | 'light'>(
    stored === 'light' ? 'light' : 'dark', // 默认沉浸深色
  )

  function apply() {
    document.documentElement.setAttribute('data-theme', theme.value)
    try {
      localStorage.setItem(KEY, theme.value)
    } catch {
      /* ignore */
    }
  }

  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    apply()
  }

  apply()

  return { theme, toggle }
})
