import { createRouter, createWebHistory } from 'vue-router'
import { loadTokens, hasToken } from '@/api/http'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/feed',
    },
    {
      path: '/feed',
      name: 'feed',
      component: () => import('@/views/FeedView.vue'),
      meta: { title: '发现' },
    },
    {
      path: '/discover',
      name: 'discover',
      component: () => import('@/views/DiscoverView.vue'),
      meta: { title: '热榜' },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: '登录' },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: '注册' },
    },
    {
      path: '/profile/:id?',
      name: 'profile',
      component: () => import('@/views/ProfileView.vue'),
      meta: { title: '主页' },
    },
    {
      path: '/upload',
      name: 'upload',
      component: () => import('@/views/UploadView.vue'),
      meta: { title: '发布', requiresAuth: true },
    },
    {
      path: '/notifications',
      name: 'notifications',
      component: () => import('@/views/NotificationsView.vue'),
      meta: { title: '消息', requiresAuth: true },
    },
    {
      path: '/video/:id',
      name: 'videoDetail',
      component: () => import('@/views/VideoDetailView.vue'),
      meta: { title: '视频' },
    },
    { path: '/:pathMatch(.*)*', redirect: '/feed' },
  ],
})

router.beforeEach((to) => {
  loadTokens()
  const authed = hasToken()
  document.title = to.meta?.title ? `${to.meta.title} · Feedi` : 'Feedi · 沉浸式短视频'

  if (to.meta?.requiresAuth && !authed) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if ((to.name === 'login' || to.name === 'register') && authed) {
    return { name: 'feed' }
  }
  return true
})

export default router
