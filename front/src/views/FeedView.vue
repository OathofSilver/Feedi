<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import type { FeedVideoItem } from '@/api/types'
import { feedLatest } from '@/api/feed'
import { likeAction, unlike } from '@/api/like'
import { makeDemoItems } from '@/api/demo'
import { useInfiniteFeed } from '@/composables/useInfiniteFeed'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useToastStore, toastErr } from '@/stores/toast'
import { useNotificationStore } from '@/stores/notification'
import VideoSlide from '@/components/feed/VideoSlide.vue'
import CommentPanel from '@/components/comments/CommentPanel.vue'
import DyIcon from '@/components/common/DyIcon.vue'
import Avatar from '@/components/common/Avatar.vue'

const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()
const toast = useToastStore()
const notif = useNotificationStore()

// 真实数据 + 后端未启动/空/出错时演示兜底（保证沉浸页始终可演示）
const usingDemo = ref(false)
function demoPage() {
  usingDemo.value = true
  return { video_list: makeDemoItems(8), has_more: false } as any
}
const feed = useInfiniteFeed(async (cursorObj) => {
  try {
    const res = await feedLatest({
      limit: 8,
      latest_time: cursorObj.next_time ?? 0,
    })
    if (res.video_list?.length) return res
    // 空数据且尚未用过演示 -> 兜底；已经翻过页则到此结束
    if (!usingDemo.value && feed.list.value.length === 0) return demoPage()
    return res
  } catch {
    // 后端不可达：首次兜底演示；否则静默
    if (!usingDemo.value && feed.list.value.length === 0) return demoPage()
    throw new Error('无法连接后端服务')
  }
})

const scroller = ref<HTMLElement | null>(null)
const currentIndex = ref(0)
const globalMuted = ref(true)
const commentFor = ref<{ videoId: number | null; open: boolean }>({ videoId: null, open: false })
const commentVisible = ref(false)
const followTargets = ref<Set<number>>(new Set()) // 已关注的作者 id（本地乐观）

const items = computed(() => feed.list.value)

// 当前可视项：全屏 slide，直接用 scrollTop / clientHeight 计算最稳
function onScroll() {
  const el = scroller.value
  if (!el) return
  const idx = Math.round(el.scrollTop / el.clientHeight)
  if (idx !== currentIndex.value) currentIndex.value = idx
  // 触底自动加载
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 240) {
    feed.loadMore()
  }
}

async function toggleLike(item: FeedVideoItem) {
  if (!auth.isAuthed) {
    toast.push('请先登录', 'info')
    return
  }
  const prev = item.is_liked
  const prevCount = item.likes_count
  // 乐观更新
  item.is_liked = !prev
  item.likes_count = prevCount + (prev ? -1 : 1)
  try {
    if (!prev) await likeAction(item.id)
    else await unlike(item.id)
  } catch (e) {
    // 回滚
    item.is_liked = prev
    item.likes_count = prevCount
    toastErr(e)
  }
}

function openComments(item: FeedVideoItem) {
  commentVisible.value = true
  commentFor.value = { videoId: item.id, open: true }
}
function closeComments() {
  commentVisible.value = false
  commentFor.value = { videoId: null, open: false }
}

function toggleFollow(item: FeedVideoItem) {
  if (!auth.isAuthed) {
    toast.push('请先登录', 'info')
    return
  }
  const id = item.author.id
  if (followTargets.value.has(id)) {
    followTargets.value.delete(id)
    toast.push('已取消关注', 'info')
  } else {
    followTargets.value.add(id)
    toast.push('关注成功', 'success')
  }
  followTargets.value = new Set(followTargets.value)
}

function openAuthor(item: FeedVideoItem) {
  router.push({ name: 'profile', params: { id: String(item.author.id) } })
}

function goSearch() {
  router.push('/discover')
}

onMounted(() => {
  feed.refresh()
})

</script>

<template>
  <div class="feed-shell">
    <!-- 沉浸滚动容器 -->
    <div ref="scroller" class="scroller" @scroll.passive="onScroll">
      <div v-for="(it, i) in items" :key="it.id" class="slide-wrap" :data-idx="i">
        <VideoSlide
          :item="it"
          :active="i === currentIndex"
          :global-muted="globalMuted"
          @toggle-like="toggleLike(it)"
          @open-comments="openComments(it)"
          @open-author="openAuthor(it)"
          @toggle-follow="toggleFollow(it)"
        />
      </div>

      <div v-if="feed.loading" class="load-hint">加载中…</div>
      <div v-else-if="feed.done && items.length" class="load-hint dim">已经到底啦</div>
      <div v-else-if="feed.error && !items.length" class="empty-err">{{ feed.error }}</div>
    </div>

    <!-- 顶部悬浮导航 -->
    <header class="topnav" :class="{ isDark: true }">
      <div class="tn-left">
        <router-link to="/feed" class="logo-link">
          <span class="logo-mark">
            <svg viewBox="0 0 64 64" width="30" height="30">
              <path d="M26 15v20.4a8.6 8.6 0 1 1-3.2-6.8V15h6.4z" fill="#25f4ee" />
              <path
                d="M44 28.8v5.2a8.6 8.6 0 1 1-8.6-8.6c.9 0 1.7.1 2.6.4v3.5a4.8 4.8 0 1 0 3.4 4.6V22.7c.9.8 2 1.6 3.3 2.2 1 .5 2.3.9 3.6 1-.4 1-.9 1.9-1.6 2.6z"
                fill="#fe2c55"
              />
            </svg>
          </span>
          <span class="word">Feedi</span>
        </router-link>
        <nav class="nav-tabs">
          <router-link to="/feed" class="tab" :class="{ on: true }">推荐</router-link>
          <router-link to="/discover" class="tab">热榜</router-link>
        </nav>
      </div>

      <div class="tn-right">
        <button class="tn-icon" title="主题切换" @click="theme.toggle">
          <span class="sun">{{ theme.theme === 'dark' ? '☀️' : '🌙' }}</span>
        </button>
        <button class="tn-icon" title="搜索/发现" @click="goSearch"><DyIcon name="search" :size="22" /></button>

        <template v-if="auth.isAuthed">
          <router-link to="/upload" class="tn-btn">发布</router-link>
          <router-link to="/notifications" class="tn-icon bell">
            <DyIcon name="bell" :size="22" />
            <span v-if="notif.unreadCount" class="badge">{{ notif.unreadCount }}</span>
          </router-link>
          <router-link :to="`/profile/${auth.accountId}`" class="tn-avatar">
            <Avatar :seed="auth.accountId ?? 0" :name="auth.username" :src="undefined" :size="34" />
          </router-link>
        </template>
        <template v-else>
          <router-link to="/login" class="tn-btn">登录</router-link>
        </template>
      </div>
    </header>

    <!-- 评论抽屉 -->
    <CommentPanel
      :show="commentVisible"
      :video-id="commentFor.videoId"
      @close="closeComments"
    />
  </div>
</template>

<style scoped>
.feed-shell {
  position: relative;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: #000;
}
.scroller {
  position: absolute;
  inset: 0;
  overflow-y: auto;
  scroll-snap-type: y mandatory;
  scrollbar-width: none;
  -ms-overflow-style: none;
  background: #000;
}
.scroller::-webkit-scrollbar {
  display: none;
}
.slide-wrap {
  width: 100%;
  height: 100vh;
  scroll-snap-align: start;
  scroll-snap-stop: always;
  position: relative;
}
.load-hint {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.7);
  font-size: 13px;
}
.empty-err {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

/* 顶部导航：叠加在沉浸层上，始终白字 */
.topnav {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 900;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 26px;
  color: #fff;
  pointer-events: none;
  background: linear-gradient(to bottom, rgba(0, 0, 0, 0.5), transparent);
}
.topnav > * {
  pointer-events: auto;
}
.tn-left {
  display: flex;
  align-items: center;
  gap: 22px;
}
.logo-link {
  display: flex;
  align-items: center;
  gap: 8px;
}
.logo-mark {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  background: #161823;
  display: flex;
  align-items: center;
  justify-content: center;
}
.word {
  font-size: 20px;
  font-weight: 800;
  letter-spacing: -0.3px;
}
.nav-tabs {
  display: flex;
  gap: 6px;
  background: rgba(255, 255, 255, 0.14);
  border-radius: var(--radius-full);
  padding: 3px;
  backdrop-filter: blur(6px);
}
.tab {
  padding: 6px 16px;
  border-radius: var(--radius-full);
  font-size: 14px;
  color: rgba(255, 255, 255, 0.8);
  transition: all 0.2s;
}
.tab.on {
  background: #fff;
  color: #161823;
  font-weight: 700;
}
.tn-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.tn-icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.14);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  position: relative;
}
.bell {
  font-size: 14px;
}
.badge {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: var(--radius-full);
  background: var(--brand-red);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}
.tn-btn {
  height: 40px;
  padding: 0 20px;
  border-radius: var(--radius-full);
  background: linear-gradient(90deg, var(--brand-red), #f04f7a);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
}
.tn-avatar {
  border: 2px solid rgba(255, 255, 255, 0.5);
  border-radius: 50%;
}
</style>
