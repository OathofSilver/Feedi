<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { videoDetail, deleteVideo } from '@/api/video'
import { accountInfo } from '@/api/account'
import { likeAction, unlike, isLiked } from '@/api/like'
import type { Video } from '@/api/types'
import { demoGradient, fmtTime, fmtCount } from '@/api/normalize'
import { useAuthStore } from '@/stores/auth'
import { toast, toastErr } from '@/stores/toast'
import CommentPanel from '@/components/comments/CommentPanel.vue'
import DyIcon from '@/components/common/DyIcon.vue'
import Avatar from '@/components/common/Avatar.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const video = ref<Video | null>(null)
const loading = ref(true)
const videoErr = ref(false)
const liked = ref(false)
const likeCount = ref(0)
const coverFailed = ref(false)
const commentOpen = ref(false)
const videoEl = ref<HTMLVideoElement | null>(null)
const authorAvatar = ref<string | undefined>(undefined)

const grad = () => (video.value ? demoGradient(video.value.id) : '')
const coverSrc = () => (video.value && video.value.cover_url && !coverFailed.value ? video.value.cover_url : '')

async function load() {
  const id = Number(route.params.id)
  if (!id) return
  loading.value = true
  video.value = null
  videoErr.value = false
  coverFailed.value = false
  commentOpen.value = false
  liked.value = false
  likeCount.value = 0
  try {
    const v = await videoDetail(id)
    video.value = v
    likeCount.value = v.likes_count
    authorAvatar.value = undefined
    // detail 不含作者头像，单独查一次作者公开资料
    accountInfo(v.author_id)
      .then((a) => {
        authorAvatar.value = a.avatar_url
      })
      .catch(() => {
        /* 保留占位 */
      })
    // 注：detail 未返回 is_liked，登录用户额外查询一次真实点赞态；
    // 游客一律视为未点赞（点击后引导登录）。
    liked.value = false
    if (auth.isAuthed) {
      try {
        liked.value = (await isLiked(id)).is_liked
      } catch {
        /* ignore */
      }
    }
  } catch (e) {
    toastErr(e)
  } finally {
    loading.value = false
  }
}

// 同一详情路由参数变化（组件复用）时重新加载，避免内容与 URL 不一致
watch(() => route.params.id, load, { immediate: true })

async function doLike() {
  if (!auth.isAuthed) {
    toast('请先登录', 'info')
    router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  const v = video.value
  if (!v) return
  const prev = liked.value
  liked.value = !prev
  likeCount.value += prev ? -1 : 1
  try {
    if (prev) await unlike(v.id)
    else await likeAction(v.id)
  } catch (e) {
    liked.value = prev
    likeCount.value -= prev ? -1 : 1
    toastErr(e)
  }
}

async function remove() {
  const v = video.value
  if (!v) return
  try {
    await deleteVideo(v.id)
    toast('已删除', 'success')
    router.replace('/feed')
  } catch (e) {
    toastErr(e)
  }
}

function togglePlay() {
  const el = videoEl.value
  if (!el) return
  if (el.paused) el.play()
  else el.pause()
}
</script>

<template>
  <div class="detail" v-if="video">
    <div class="stage" :style="{ background: grad() }">
      <img v-if="coverSrc()" :src="coverSrc()" class="cover" alt="" @error="coverFailed = true" />
      <video
        v-if="video.play_url"
        ref="videoEl"
        :src="video.play_url"
        class="player"
        playsinline
        controls
        autoplay
        @error="videoErr = true"
      ></video>
      <div v-if="videoErr || !video.play_url" class="err">
        <DyIcon name="play" :size="40" />
        <p>视频源暂不可用 · 已加载元数据</p>
      </div>
    </div>

    <button class="back" @click="router.back()"><DyIcon name="arrowLeft" :size="22" /></button>

    <div class="info">
      <h1 class="title">{{ video.title }}</h1>
      <p class="desc">{{ video.description }}</p>

      <div class="author-row">
        <div class="au" @click="router.push({ name: 'profile', params: { id: String(video.author_id) } })">
          <Avatar :seed="video.author_id" :name="video.username" :src="authorAvatar" :size="44" />
          <div>
            <b>@{{ video.username }}</b>
            <span>{{ fmtTime(video.create_time) }}</span>
          </div>
        </div>
        <div class="acts">
          <button class="act" :class="{ on: liked }" @click="doLike">
            <DyIcon name="heart" :size="24" :active="liked" />
            {{ fmtCount(likeCount) }}
          </button>
          <button class="act" @click="commentOpen = true">
            <DyIcon name="comment" :size="24" />评论
          </button>
          <button v-if="auth.isAuthed && auth.username === video.username" class="act danger" @click="remove">
            <DyIcon name="trash" :size="20" />删除
          </button>
        </div>
      </div>
    </div>

    <CommentPanel :show="commentOpen" :video-id="video.id" @close="commentOpen = false" />
  </div>
  <div v-else-if="loading" class="loading dim">加载中…</div>
  <div v-else class="loading dim">视频不存在</div>
</template>

<style scoped>
.detail {
  height: 100%;
  width: 100%;
  background: var(--bg);
  overflow-y: auto;
  color: var(--text-1);
}
.stage {
  position: relative;
  height: 62vh;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.cover,
.player {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #000;
}
.cover {
  object-fit: cover;
}
.err {
  position: relative;
  color: rgba(255, 255, 255, 0.9);
  text-align: center;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
.err p {
  margin: 0;
  font-size: 14px;
  padding: 6px 14px;
  background: rgba(0, 0, 0, 0.4);
  border-radius: var(--radius-full);
}
.back {
  position: fixed;
  top: 16px;
  left: 16px;
  z-index: 30;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.45);
  color: #fff;
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
}
.info {
  padding: 20px 4vw 60px;
}
.title {
  font-size: 22px;
  margin: 0 0 6px;
}
.desc {
  color: var(--text-2);
  margin: 0 0 20px;
  font-size: 14px;
  line-height: 1.6;
}
.author-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 16px 0;
  border-top: 1px solid var(--line);
}
.au {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}
.au div b {
  display: block;
  font-size: 15px;
}
.au div span {
  font-size: 12px;
  color: var(--text-3);
}
.acts {
  display: flex;
  gap: 8px;
}
.act {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: var(--radius-full);
  background: var(--surface);
  border: 1px solid var(--line);
  font-size: 13px;
  color: var(--text-1);
}
.act.on {
  color: var(--danger);
}
.act.danger {
  color: var(--danger);
}
.loading {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
}
</style>
