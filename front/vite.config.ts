import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // 后端 API 统一代理，规避 CORS
      '/api': {
        target: 'http://localhost:9000',
        changeOrigin: true,
      },
      // 头像等静态资源
      '/static': {
        target: 'http://localhost:9000',
        changeOrigin: true,
      },
    },
  },
})
