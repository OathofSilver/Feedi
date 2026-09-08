<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { CommentItem } from '@/api/types'
import { listComments, publishComment, deleteComment } from '@/api/comment'
import { fmtTime } from '@/api/normalize'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'
import { toast, toastErr } from '@/stores/toast'
import DyIcon from '@/components/common/DyIcon.vue'
import Avatar from '@/components/common/Avatar.vue'

const props = defineProps<{ show: boolean; videoId: number | null; videoAuthorName?: string }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const auth = useAuthStore()
const router = useRouter()

const comments = ref<CommentItem[]>([])
const loading = ref(false)
const text = ref('')
const sending = ref(false)
const commentSeq = ref(0)
let lastFetchedVideo: number | null = null

watch(
  () => [props.show, props.videoId] as const,
  async ([show, vid]) => {
    if (show && vid && vid !== lastFetchedVideo) {
      lastFetchedVideo = vid
      await load(vid)
    }
  },
  { immediate: true },
)

async function load(vid: number) {
  loading.value = true
  try {
    comments.value = await listComments(vid)
  } catch (e) {
    toastErr(e)
  } finally {
    loading.value = false
  }
}

async function send() {
  const c = text.value.trim()
  if (!c || !props.videoId) return
  sending.value = true
  try {
    await publishComment(props.videoId, c)
    text.value = ''
    toast('评论成功', 'success')
    comments.value.unshift({
      id: ++commentSeq.value * -1, // 本地临时
      username: auth.username || 'me',
      video_id: props.videoId,
      author_id: auth.accountId ?? 0,
      content: c,
      created_at: new Date().toISOString(),
    })
  } catch (e) {
    toastErr(e)
  } finally {
    sending.value = false
  }
}

async function del(c: CommentItem) {
  if (c.id > 0) {
    try {
      await deleteComment(c.id)
    } catch (e) {
      toastErr(e)
      return
    }
  }
  comments.value = comments.value.filter((x) => x !== c)
}

function openCommentsRequireLogin() {
  if (!auth.isAuthed) {
    toast('请先登录', 'info')
    router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
    return false
  }
  return true
}
</script>

<template>
  <Teleport to="body">
    <Transition name="panel">
      <div v-if="show" class="cp-mask" @click.self="emit('close')">
        <aside class="cp-panel">
          <header class="cp-head">
            <span class="cp-title">评论</span>
            <button class="cp-x" aria-label="关闭" @click="emit('close')">✕</button>
          </header>

          <div class="cp-body hide-scroll">
            <template v-if="comments.length">
              <div v-for="c in comments" :key="c.id" class="cm">
                <Avatar :seed="c.author_id" :name="c.username" :size="36" />
                <div class="cm-main">
                  <span class="cm-name">{{ c.username }}</span>
                  <p class="cm-text">{{ c.content }}</p>
                  <span class="cm-time">{{ fmtTime(c.created_at) }}</span>
                </div>
                <button v-if="auth.isAuthed && auth.username === c.username" class="cm-del" @click="del(c)">
                  <DyIcon name="trash" :size="15" />
                </button>
              </div>
            </template>
            <div v-else-if="loading" class="empty dim">加载中…</div>
            <div v-else class="empty dim">还没有评论，来抢沙发</div>
          </div>

          <footer class="cp-foot">
            <input
              v-model="text"
              :placeholder="auth.isAuthed ? '友善评论，理性发言' : '登录后参与评论'"
              @keyup.enter="openCommentsRequireLogin() && send()"
            />
            <button
              class="send"
              :disabled="sending || !text.trim()"
              @click="openCommentsRequireLogin() && send()"
            >
              发布
            </button>
          </footer>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.cp-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 1200;
  display: flex;
  justify-content: flex-end;
}
.cp-panel {
  width: 400px;
  max-width: 92vw;
  height: 100%;
  background: var(--bg-elev);
  border-left: 1px solid var(--line);
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-2);
}
.cp-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px;
  border-bottom: 1px solid var(--line);
}
.cp-title {
  font-weight: 700;
  font-size: 16px;
}
.cp-x {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: var(--surface-2);
  color: var(--text-2);
  font-size: 14px;
}
.cp-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}
.cm {
  display: flex;
  gap: 12px;
  padding: 12px 20px;
}
.cm:hover .cm-del {
  opacity: 1;
}
.cm-main {
  flex: 1;
  min-width: 0;
}
.cm-name {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-2);
}
.cm-text {
  margin: 4px 0 2px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
}
.cm-time {
  font-size: 12px;
  color: var(--text-3);
}
.cm-del {
  opacity: 0;
  color: var(--text-3);
  align-self: flex-start;
  transition: opacity 0.2s;
}
.cp-foot {
  display: flex;
  gap: 10px;
  padding: 12px 16px;
  border-top: 1px solid var(--line);
  background: var(--bg-elev);
}
.cp-foot input {
  flex: 1;
  height: 40px;
  border-radius: var(--radius-full);
  border: 1px solid var(--line);
  background: var(--surface-2);
  padding: 0 16px;
  font-size: 14px;
  outline: none;
}
.send {
  height: 40px;
  padding: 0 20px;
  border-radius: var(--radius-full);
  background: var(--brand-red);
  color: #fff;
  font-weight: 600;
  font-size: 14px;
}
.send:disabled {
  opacity: 0.5;
  cursor: default;
}
.empty {
  text-align: center;
  padding: 60px 20px;
  font-size: 14px;
}
.panel-enter-active,
.panel-leave-active {
  transition: opacity 0.25s ease;
}
.panel-enter-active .cp-panel,
.panel-leave-active .cp-panel {
  transition: transform 0.25s ease;
}
.panel-enter-from,
.panel-leave-to {
  opacity: 0;
}
.panel-enter-from .cp-panel,
.panel-leave-to .cp-panel {
  transform: translateX(100%);
}
</style>
