import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useToastStore } from './stores/toast'
import { useThemeStore } from './stores/theme'
import './styles/base.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

// 启动时即实例化 theme，让 data-theme 在所有路由生效（包括 Login/Discover 等不直接使用主题 store 的页）
useThemeStore(pinia)

// 暴露 toast 供非组件调用（toast.ts 里用）
const toastStore = useToastStore(pinia)
;(window as unknown as { __toast?: { push: (t: string, k: 'info' | 'success' | 'error') => void } }).__toast = {
  push: (t, k) => toastStore.push(t, k),
}

app.mount('#app')
