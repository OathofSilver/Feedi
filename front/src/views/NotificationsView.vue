<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import type { NotificationItem } from '@/api/types'
import { useNotificationStore } from '@/stores/notification'
import { fmtTime } from '@/api/normalize'
import { useAuthStore } from '@/stores/auth'
import DyIcon from '@/components/common/DyIcon.vue'
import Avatar from '@/components/common/Avatar.vue'

const router = useRouter()
const auth = useAuthStore()
const notif = useNotificationStore()

const typeText: Record<string, string> = {
  like: '赞了你的视频',
  comment: '评论了你的视频',
  follow: '关注了你',
}

function open(n: NotificationItem) {
  if (n.video_id) router.push(`/video/${n.video_id}`)
  else if (n.type === 'follow') router.push(`/profile/${n.actor_id}`)
}

onMounted(() => {
  // 视为已读（清未读 + 记录水位），并拉取最新列表
  notif.markSeen()
  notif.ensureList()
})
onBeforeUnmount(() => {
  notif.closePanel()
})
</script>

<template>
  <div class="ntf">
    <header class="head">
      <button class="back" @click="router.push('/feed')"><DyIcon name="arrowLeft" :size="22" /></button>
      <h1>消息通知</h1>
    </header>

    <p v-if="!auth.isAuthed" class="empty dim">登录后查看通知</p>
    <div v-else-if="notif.notifList.length" class="list">
      <div v-for="n in notif.notifList" :key="n.id" class="item" @click="open(n)">
        <Avatar :seed="n.actor_id" :name="n.actor_name" :src="undefined" :size="46" />
        <div class="main">
          <p>
            <b>@{{ n.actor_name }}</b>
            <span class="dim"> {{ typeText[n.type] || n.type }}</span>
          </p>
          <p v-if="n.content" class="quote">{{ n.content }}</p>
          <span class="time dim">{{ fmtTime(n.created_at) }}</span>
        </div>
        <span class="dot" v-if="n.type === 'like'" :title="n.type">❤️</span>
        <span class="dot" v-else-if="n.type === 'comment'" :title="n.type">💬</span>
        <span class="dot" v-else-if="n.type === 'follow'" :title="n.type">👤</span>
      </div>
    </div>
    <div v-else class="empty dim">暂无消息</div>
  </div>
</template>

<style scoped>
.ntf {
  height: 100%;
  overflow-y: auto;
  background: var(--bg);
  color: var(--text-1);
}
.head {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px 24px;
}
.back {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: var(--surface);
  box-shadow: var(--shadow-1);
  display: flex;
  align-items: center;
  justify-content: center;
}
.head h1 {
  margin: 0;
  font-size: 20px;
}
.list {
  padding: 0 16px 40px;
}
.item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 8px;
  cursor: pointer;
  border-bottom: 1px solid var(--line);
}
.main {
  flex: 1;
  min-width: 0;
}
.main p {
  margin: 2px 0;
  font-size: 14px;
  line-height: 1.4;
}
.main .quote {
  color: var(--text-2);
  font-size: 13px;
  background: var(--surface-2);
  padding: 6px 10px;
  border-radius: 8px;
}
.time {
  font-size: 12px;
}
.dot {
  font-size: 18px;
}
.empty {
  text-align: center;
  padding: 80px 0;
  font-size: 15px;
}
</style>
