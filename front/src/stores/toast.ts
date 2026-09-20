import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface ToastItem {
  id: number
  text: string
  kind: 'info' | 'success' | 'error'
}

let seq = 0

export const useToastStore = defineStore('toast', () => {
  const toasts = ref<ToastItem[]>([])

  function push(text: string, kind: ToastItem['kind'] = 'info', ttl = 2600) {
    const id = ++seq
    toasts.value.push({ id, text, kind })
    setTimeout(() => remove(id), ttl)
    return id
  }

  function remove(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  return { toasts, push, remove }
})

/** 全局便捷调用（供非组件/组件通用） */
export function toast(text: string, kind: ToastItem['kind'] = 'info') {
  // 在 main.ts 中把 store 注册到全局
  const global = (window as unknown as { __toast?: { push: (t: string, k: ToastItem['kind']) => void } }).__toast
  global?.push(text, kind)
}

/** 把 Error.message 透传为 error toast */
export function toastErr(e: unknown) {
  const msg = e instanceof Error ? e.message : String(e)
  toast(msg, 'error')
}
