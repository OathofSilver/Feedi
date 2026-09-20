<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { Account, Video, FollowerItem } from '@/api/types'
import { accountInfo, uploadAvatar, updateProfile } from '@/api/account'
import { myLikedVideos } from '@/api/like'
import { socialCounts, follow, unfollow, myFollowers, myVloggers, isFollowing } from '@/api/social'
import { demoGradient } from '@/api/normalize'
import { useAuthStore } from '@/stores/auth'
import { toast, toastErr } from '@/stores/toast'
import DyIcon from '@/components/common/DyIcon.vue'
import Avatar from '@/components/common/Avatar.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const uid = computed(() => Number(route.params.id ?? auth.accountId ?? 0))
const isOwn = computed(() => auth.isAuthed && uid.value === auth.accountId)

const acc = ref<Account | null>(null)
const followers = ref<FollowerItem[]>([])
const vloggers = ref<FollowerItem[]>([])
const following = ref<FollowerItem[]>([])
const counts = ref<{ follower_count: number; vlogger_count: number } | null>(null)
const likedVideos = ref<Video[]>([])
const tab = ref<'liked' | 'followers' | 'following'>('liked')
const coverFailed = ref<Record<number, boolean>>({})

// 我关注了这个作者吗（本地乐观）
const followedMe = ref(false)
const editingBio = ref(false)
const bioDraft = ref('')
const avatarInput = ref<HTMLInputElement | null>(null)

async function load() {
  // 计数/关系数据必须按「被查看用户」拉取，避免串成 viewer 自己的数据
  try {
    const a = await accountInfo(uid.value)
    acc.value = a
  } catch (e) {
    toastErr(e)
  }
  // 关注态：他人主页查询当前登录者与 ta 的关系；自己的主页恒为 false
  if (auth.isAuthed && !isOwn.value) {
    try {
      const r = await isFollowing(uid.value)
      followedMe.value = r.is_following
    } catch {
      followedMe.value = false
    }
  } else {
    followedMe.value = false
  }
  // 计数（登录用户可查任意用户；游客取不到则保持 null，UI 显示占位）
  try {
    counts.value = await socialCounts(uid.value)
  } catch {
    counts.value = null
  }
}

async function toggleFollowMe() {
  if (!auth.isAuthed) {
    toast('请先登录', 'info')
    return
  }
  const target = uid.value
  const prev = followedMe.value
  // 乐观更新
  followedMe.value = !prev
  if (counts.value) {
    counts.value = { ...counts.value, follower_count: counts.value.follower_count + (followedMe.value ? 1 : -1) }
  }
  try {
    if (!prev) await follow(target)
    else await unfollow(target)
    toast(followedMe.value ? '关注成功' : '已取消关注', followedMe.value ? 'success' : 'info')
  } catch (e) {
    // 回滚：按 prev 方向撤销（勿用翻转后的值计算，否则符号反）
    followedMe.value = prev
    if (counts.value) {
      counts.value = { ...counts.value, follower_count: counts.value.follower_count + (prev ? 1 : -1) }
    }
    toastErr(e)
  }
}

async function loadLiked() {
  if (!isOwn.value) return
  try {
    likedVideos.value = await myLikedVideos()
  } catch {
    /* ignore */
  }
}

async function pickTab(t: typeof tab.value) {
  tab.value = t
  if (t === 'followers') {
    followers.value = []
    if (!auth.isAuthed) return
    try {
      followers.value = (await myFollowers(uid.value)).followers
    } catch {
      /* ignore */
    }
  } else if (t === 'following') {
    following.value = []
    if (!auth.isAuthed) return
    try {
      const v = (await myVloggers(uid.value)).vloggers
      vloggers.value = v
      following.value = v
    } catch {
      /* ignore */
    }
  } else {
    loadLiked()
  }
}

function openBioEditor() {
  if (!isOwn.value) return
  bioDraft.value = acc.value?.bio || ''
  editingBio.value = true
}
async function saveBio() {
  try {
    await updateProfile({ bio: bioDraft.value })
    if (acc.value) acc.value.bio = bioDraft.value
    editingBio.value = false
    toast('已保存', 'success')
  } catch (e) {
    toastErr(e)
  }
}
/** bio 编辑框回车：跳过中文输入法选词阶段的 Enter */
function onBioEnter(e: KeyboardEvent) {
  if (e.isComposing || e.keyCode === 229) return
  saveBio()
}
async function onAvatarPick(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  try {
    const res = await uploadAvatar(file)
    if (acc.value) acc.value.avatar_url = res.url
    // 同步会话资料，顶栏头像立即生效
    auth.setProfileAvatar(res.url)
    toast('头像已更新', 'success')
  } catch (err) {
    toastErr(err)
  }
}

watch(uid, () => {
  acc.value = null
  followers.value = []
  vloggers.value = []
  following.value = []
  likedVideos.value = []
  tab.value = 'liked'
  followedMe.value = false
  counts.value = null
  // 无有效目标用户（未登录且无 id 参数）→ 引导登录
  if (uid.value <= 0) {
    if (!auth.isAuthed) {
      router.replace({ name: 'login', query: { redirect: route.fullPath } })
    }
    return
  }
  load()
  loadLiked()
}, { immediate: true })
</script>

<template>
  <div class="prof">
    <header class="head">
      <button class="back" @click="router.back()"><DyIcon name="arrowLeft" :size="22" /></button>
      <router-link to="/feed" class="home"><DyIcon name="home" :size="22" /></router-link>
      <div class="sp" />
      <router-link to="/discover" class="btn"><DyIcon name="search" :size="18" /></router-link>
      <router-link v-if="auth.isAuthed" to="/upload" class="btn">发布</router-link>
    </header>

    <template v-if="acc">
      <section class="hero">
        <div class="avatar-wrap" @click="isOwn && avatarInput?.click()">
          <Avatar :seed="acc.id" :name="acc.username" :src="acc.avatar_url || undefined" :size="88" />
          <span v-if="isOwn" class="cam">📷</span>
        </div>
        <input
          ref="avatarInput"
          type="file"
          accept="image/*"
          style="display: none"
          @change="onAvatarPick"
        />
        <h1 class="name">@{{ acc.username }}</h1>
        <p class="bio">
          <template v-if="editingBio">
            <input v-model="bioDraft" class="bio-in" placeholder="写点什么介绍自己…" @keyup.enter="onBioEnter" />
            <button class="bio-save" @click="saveBio">保存</button>
          </template>
          <template v-else>
            {{ acc.bio || '这个人很懒，什么都没写～' }}
            <button v-if="isOwn" class="edit-bio" @click="openBioEditor">✏️ 编辑</button>
          </template>
        </p>

        <div class="stats">
          <div class="stat">
            <b>{{ counts?.vlogger_count ?? '—' }}</b><span>关注</span>
          </div>
          <div class="stat">
            <b>{{ counts?.follower_count ?? '—' }}</b><span>粉丝</span>
          </div>
          <div class="stat">
            <b>{{ isOwn ? likedVideos.length : '-' }}</b><span>喜欢</span>
          </div>
        </div>

        <div v-if="!isOwn && auth.isAuthed" class="acts">
          <button class="follow" :class="{ on: followedMe }" @click="toggleFollowMe">
            {{ followedMe ? '已关注' : '+ 关注' }}
          </button>
        </div>
        <div v-else-if="!isOwn && !auth.isAuthed" class="acts">
          <router-link to="/login" class="follow">+ 关注</router-link>
        </div>
      </section>

      <nav class="tabs">
        <button :class="{ on: tab === 'liked' }" @click="pickTab('liked')">喜欢</button>
        <button :class="{ on: tab === 'followers' }" @click="pickTab('followers')">粉丝</button>
        <button :class="{ on: tab === 'following' }" @click="pickTab('following')">关注</button>
      </nav>

      <section class="content">
        <!-- 喜欢的视频（仅自己可查） -->
        <template v-if="tab === 'liked'">
          <div v-if="!isOwn" class="tip dim">他人作品列表暂不对外展示</div>
          <div v-else-if="likedVideos.length" class="grid">
            <router-link
              v-for="v in likedVideos"
              :key="v.id"
              :to="`/video/${v.id}`"
              class="cell"
            >
              <div class="thumb" :style="{ backgroundImage: `url('${v.cover_url && !coverFailed[v.id] ? v.cover_url : demoGradient(v.id)}')` }">
                <span class="like"><DyIcon name="heart" :size="14" />{{ v.likes_count }}</span>
              </div>
            </router-link>
          </div>
          <div v-else class="tip dim">还没有喜欢的视频</div>
        </template>

        <!-- 粉丝 -->
        <template v-else-if="tab === 'followers'">
          <div v-if="!auth.isAuthed" class="tip dim">登录后查看粉丝</div>
          <div v-else-if="followers.length" class="list">
            <div v-for="f in followers" :key="f.id" class="row">
              <router-link :to="`/profile/${f.id}`" class="ri">
                <Avatar :seed="f.id" :name="f.username" :src="f.avatar_url || undefined" :size="44" />
                <div>
                  <b>@{{ f.username }}</b>
                  <span>{{ f.bio || '暂无简介' }}</span>
                </div>
              </router-link>
            </div>
          </div>
          <div v-else class="tip dim">暂无粉丝</div>
        </template>

        <!-- 关注 -->
        <template v-else>
          <div v-if="!auth.isAuthed" class="tip dim">登录后查看关注</div>
          <div v-else-if="vloggers.length" class="list">
            <div v-for="f in vloggers" :key="f.id" class="row">
              <router-link :to="`/profile/${f.id}`" class="ri">
                <Avatar :seed="f.id" :name="f.username" :src="f.avatar_url || undefined" :size="44" />
                <div>
                  <b>@{{ f.username }}</b>
                  <span>{{ f.bio || '暂无简介' }}</span>
                </div>
              </router-link>
            </div>
          </div>
          <div v-else class="tip dim">还没有关注任何人</div>
        </template>
      </section>
    </template>
    <div v-else class="tip dim loading">加载中…</div>
  </div>
</template>

<style scoped>
.prof {
  height: 100%;
  overflow-y: auto;
  background: var(--bg);
  color: var(--text-1);
}
.head {
  position: sticky;
  top: 0;
  z-index: 40;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 20px;
  background: var(--bg);
  backdrop-filter: blur(8px);
}
.back,
.home,
.btn {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: var(--surface);
  box-shadow: var(--shadow-1);
  display: flex;
  align-items: center;
  justify-content: center;
}
.sp {
  flex: 1;
}
.btn {
  width: auto;
  padding: 0 12px;
  border-radius: var(--radius-full);
  color: var(--brand-red);
  font-size: 13px;
  font-weight: 600;
}
.hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 10px 20px 18px;
  text-align: center;
}
.avatar-wrap {
  position: relative;
  cursor: default;
}
.cam {
  position: absolute;
  bottom: 0;
  right: -2px;
  font-size: 18px;
  background: var(--surface);
  border-radius: 50%;
  padding: 2px;
}
.name {
  margin: 12px 0 4px;
  font-size: 22px;
}
.bio {
  color: var(--text-2);
  font-size: 14px;
  margin: 0 0 16px;
  max-width: 60vw;
}
.bio-in {
  border: 1px solid var(--line);
  background: var(--surface);
  border-radius: var(--radius-md);
  padding: 6px 10px;
  font-size: 13px;
  width: 200px;
}
.bio-save {
  margin-left: 6px;
  color: var(--brand-red);
  font-size: 13px;
}
.edit-bio {
  color: var(--text-3);
  font-size: 12px;
  margin-left: 4px;
}
.stats {
  display: flex;
  gap: 34px;
  margin-bottom: 16px;
}
.stat b {
  display: block;
  font-size: 20px;
}
.stat span {
  color: var(--text-3);
  font-size: 13px;
}
.acts .follow,
.acts .follow {
  display: inline-block;
  padding: 9px 30px;
  border-radius: var(--radius-full);
  background: linear-gradient(90deg, var(--brand-red), #f04f7a);
  color: #fff;
  font-weight: 600;
}
.acts .follow.on {
  background: var(--surface-3);
  color: var(--text-2);
}
.tabs {
  display: flex;
  justify-content: center;
  gap: 30px;
  border-bottom: 1px solid var(--line);
}
.tabs button {
  padding: 12px 4px;
  font-size: 15px;
  color: var(--text-2);
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}
.tabs button.on {
  color: var(--text-1);
  font-weight: 700;
  border-color: var(--brand-red);
}
.content {
  padding: 20px 4vw 60px;
  min-height: 30vh;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
  gap: 6px;
}
.thumb {
  aspect-ratio: 1;
  border-radius: 6px;
  background-size: cover;
  background-position: center;
  position: relative;
}
.like {
  position: absolute;
  left: 6px;
  bottom: 6px;
  color: #fff;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 3px;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
}
.list {
  display: flex;
  flex-direction: column;
}
.row {
  padding: 8px 0;
  border-bottom: 1px solid var(--line);
}
.ri {
  display: flex;
  align-items: center;
  gap: 12px;
}
.ri div b {
  display: block;
  font-size: 15px;
}
.ri div span {
  font-size: 12px;
  color: var(--text-3);
}
.tip {
  text-align: center;
  padding: 40px 0;
}
</style>
