<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { FeedVideoItem } from '@/api/types'
import { demoGradient, fmtCount, fmtTime } from '@/api/normalize'
import { useVideoPlayer } from '@/composables/useVideoPlayer'
import DyIcon from '@/components/common/DyIcon.vue'
import Avatar from '@/components/common/Avatar.vue'

const props = defineProps<{
  item: FeedVideoItem
  /** 是否为当前可视项 */
  active: boolean
  /** Feed 层下发的全局静音态 */
  globalMuted?: boolean
}>()

const emit = defineEmits<{
  (e: 'toggleLike'): void
  (e: 'openComments'): void
  (e: 'openAuthor'): void
  (e: 'toggleFollow'): void
}>()

const { videoRef, isPlaying, hasError, isMuted, play, pause, toggle, toggleMute } =
  useVideoPlayer()

const coverFailed = ref(false)
const burst = ref(0)
const coverUrl = computed(() => (props.item.cover_url && !coverFailed.value ? props.item.cover_url : ''))
const grad = computed(() => demoGradient(props.item.id))
const playUrl = computed(() => props.item.play_url)

// 跟随全局静音
watch(
  () => props.globalMuted,
  (m) => {
    if (typeof m === 'boolean') {
      isMuted.value = m
      if (videoRef.value) videoRef.value.muted = m
    }
  },
  { immediate: true },
)

// 成为可视项 -> 播放；离开 -> 暂停
watch(
  () => props.active,
  async (a) => {
    if (a) {
      await new Promise((r) => requestAnimationFrame(() => r(null)))
      play()
    } else {
      pause()
    }
  },
  { immediate: true },
)

function onDbl() {
  burst.value++
  if (!props.item.is_liked) emit('toggleLike')
}
</script>

<template>
  <div class="slide" @dblclick="onDbl">
    <div class="media-bg" :style="{ background: grad }">
      <img v-if="coverUrl" :src="coverUrl" class="poster" alt="" @error="coverFailed = true" />
      <video
        v-if="playUrl"
        ref="videoRef"
        class="player"
        :src="playUrl"
        playsinline
        loop
        preload="metadata"
      ></video>

      <!-- 无真实媒体或播放失败：演示占位文案 -->
      <div v-if="!playUrl || hasError" class="demo-cap">
        <DyIcon name="play" :size="26" />
        <span>视频源暂不可用 · 演示预览</span>
      </div>

      <!-- 点击播放控制 -->
      <button class="tap" aria-label="播放控制" @click.stop="toggle">
        <span v-if="active && !isPlaying && playUrl && !hasError" class="play-badge">
          <DyIcon name="play" :size="46" />
        </span>
      </button>

      <!-- 双击红心 burst -->
      <Transition name="burst">
        <span v-if="burst" :key="burst" class="dbl-heart">
          <DyIcon name="heart" :size="90" active />
        </span>
      </Transition>
    </div>

    <div class="bottom-grad" />

    <div class="caption">
      <div class="meta-row">
        <span class="author-name">@{{ item.author.username }}</span>
        <span class="time">{{ fmtTime(item.create_time) }}</span>
      </div>
      <p v-if="item.title" class="title">{{ item.title }}</p>
      <p v-if="item.description" class="desc"># {{ item.description }}</p>
      <div class="music-pill">
        <span class="spin"><DyIcon name="music" :size="14" /></span>
        <span class="ellipsis">{{ item.author.username }} 的原声</span>
      </div>
    </div>

    <aside class="rail">
      <div class="rail-item">
        <div class="avatar-wrap" @click="emit('openAuthor')">
          <Avatar :seed="item.author.id" :name="item.author.username" :src="undefined" :size="48" />
        </div>
        <button class="follow-btn" aria-label="关注" @click.stop="emit('toggleFollow')">+</button>
      </div>

      <div class="rail-item">
        <button class="action" :class="{ liked: item.is_liked }" aria-label="点赞" @click.stop="emit('toggleLike')">
          <DyIcon name="heart" :size="30" :active="item.is_liked" />
        </button>
        <span class="count">{{ fmtCount(item.likes_count) }}</span>
      </div>

      <div class="rail-item">
        <button class="action" aria-label="评论" @click.stop="emit('openComments')">
          <DyIcon name="comment" :size="30" />
        </button>
        <span class="count" @click.stop="emit('openComments')">评论</span>
      </div>

      <div class="rail-item">
        <button class="action" aria-label="分享"><DyIcon name="share" :size="30" /></button>
        <span class="count">分享</span>
      </div>

      <div class="rail-item">
        <button class="action" aria-label="更多"><DyIcon name="more" :size="26" /></button>
      </div>

      <div class="rail-item">
        <button class="action sound" aria-label="声音" @click.stop="toggleMute">
          <span v-if="isMuted">🔇</span>
          <span v-else>🔊</span>
        </button>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.slide {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #000;
  user-select: none;
}
.media-bg {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}
.poster,
.player {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.poster {
  z-index: 1;
}
.player {
  z-index: 2;
}
.demo-cap {
  position: relative;
  z-index: 3;
  display: flex;
  align-items: center;
  gap: 8px;
  color: rgba(255, 255, 255, 0.85);
  font-size: 14px;
  background: rgba(0, 0, 0, 0.18);
  padding: 8px 16px;
  border-radius: var(--radius-full);
  backdrop-filter: blur(2px);
}
.tap {
  position: absolute;
  inset: 0;
  z-index: 4;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}
.play-badge {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  backdrop-filter: blur(2px);
}
.bottom-grad {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 46%;
  z-index: 3;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.6), transparent);
  pointer-events: none;
}
.caption {
  position: absolute;
  left: 20px;
  right: 108px;
  bottom: 42px;
  z-index: 5;
  color: #fff;
  display: flex;
  flex-direction: column;
  gap: 8px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
  pointer-events: none;
}
.meta-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}
.author-name {
  font-weight: 700;
  font-size: 16px;
}
.time {
  color: rgba(255, 255, 255, 0.8);
}
.title {
  margin: 0;
  font-size: 15px;
  line-height: 1.4;
  font-weight: 600;
}
.desc {
  margin: 0;
  font-size: 14px;
  color: rgba(255, 255, 255, 0.9);
}
.music-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  width: fit-content;
  max-width: 100%;
  font-size: 13px;
  color: #fff;
  background: rgba(255, 255, 255, 0.16);
  border-radius: var(--radius-full);
  padding: 6px 14px;
}
.spin {
  display: inline-flex;
  animation: spin 5s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.rail {
  position: absolute;
  right: 10px;
  bottom: 20%;
  z-index: 6;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
}
.rail-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}
.avatar-wrap {
  position: relative;
  border-radius: 50%;
  padding: 2px;
  background: linear-gradient(45deg, var(--brand-red), var(--brand-cyan));
  cursor: pointer;
}
.avatar-wrap :deep(.avatar) {
  border: 2px solid #161823;
}
.follow-btn {
  width: 24px;
  height: 24px;
  margin-top: -12px;
  border-radius: 50%;
  background: var(--brand-red);
  color: #fff;
  font-size: 16px;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
}
.action {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.4));
  transition: transform 0.15s;
}
.action:hover {
  transform: scale(1.08);
}
.action:active {
  transform: scale(0.92);
}
.action.liked {
  color: var(--danger);
}
.sound {
  font-size: 20px;
}
.count {
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
  cursor: pointer;
}
.dbl-heart {
  position: absolute;
  z-index: 7;
  color: var(--danger);
  animation: pop 0.9s ease forwards;
}
@keyframes pop {
  0% {
    transform: scale(0.3);
    opacity: 0;
  }
  30% {
    transform: scale(1.25);
    opacity: 1;
  }
  100% {
    transform: scale(1.5) translateY(-30px);
    opacity: 0;
  }
}
</style>
