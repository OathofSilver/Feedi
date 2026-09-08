<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import type { FeedVideoItem } from '@/api/types'
import { feedLikesCount, feedTag } from '@/api/feed'
import { makeDemoItems } from '@/api/demo'
import { fmtCount, demoGradient } from '@/api/normalize'
import { useAuthStore } from '@/stores/auth'
import { toast } from '@/stores/toast'
import DyIcon from '@/components/common/DyIcon.vue'
import Avatar from '@/components/common/Avatar.vue'

const router = useRouter()
const auth = useAuthStore()

const hot = ref<FeedVideoItem[]>([])
const loadedHot = ref(false)
const keyword = ref('')
const searching = ref(false)
const tagResult = ref<FeedVideoItem[]>([])
const searched = ref(false)
const coverFailed = ref<Record<number, boolean>>({})

async function loadHot() {
  if (loadedHot.value) return
  loadedHot.value = true
  try {
    const res = await feedLikesCount({ limit: 24 })
    hot.value = res.video_list?.length ? res.video_list : makeDemoItems(12)
  } catch {
    // 后端未启动时用演示数据兜底
    hot.value = makeDemoItems(12)
  }
}
loadHot()

async function doSearch() {
  const kw = keyword.value.trim()
  if (!kw) return
  searching.value = true
  searched.value = true
  try {
    const res = await feedTag(kw, 24)
    tagResult.value = res.video_list?.length ? res.video_list : makeDemoItems(12)
  } catch {
    tagResult.value = makeDemoItems(12)
  } finally {
    searching.value = false
  }
}

function goVideo(id: number) {
  router.push({ name: 'videoDetail', params: { id: String(id) } })
}
function goAuthor(id: number) {
  router.push({ name: 'profile', params: { id: String(id) } })
}
function bg(it: FeedVideoItem) {
  return it.cover_url && !coverFailed.value[it.id] ? it.cover_url : demoGradient(it.id)
}
</script>

<template>
  <div class="disc">
    <header class="head">
      <router-link to="/feed" class="back"><DyIcon name="arrowLeft" :size="20" /></router-link>
      <h1>发现 · 热榜</h1>
      <div class="search">
        <DyIcon name="search" :size="18" />
        <input
          v-model="keyword"
          placeholder="搜索话题 / 标签"
          @keyup.enter="doSearch"
        />
        <button class="go" @click="doSearch">搜索</button>
      </div>
      <router-link to="/upload" v-if="auth.isAuthed" class="pub">＋ 发布</router-link>
    </header>

    <!-- 搜索结果 -->
    <section v-if="searched" class="section">
      <h2 v-if="keyword" class="stitle">
        关于“<b>{{ keyword }}</b>”的{{ searching ? '搜索中…' : '结果' }}
        <button class="reset" @click="searched = false; tagResult = []; keyword = ''">返回热榜</button>
      </h2>
      <div v-if="tagResult.length" class="grid">
        <div v-for="(it, i) in tagResult" :key="it.id" class="cell" @click="goVideo(it.id)">
          <div class="thumb" :style="{ backgroundImage: `url('${bg(it)}')` }">
            <span class="rank" v-if="false"></span>
            <span class="plays"><DyIcon name="play" :size="14" />{{ fmtCount(it.likes_count) }}</span>
          </div>
          <p class="t ellipsis">{{ it.title }}</p>
          <div class="a" @click.stop="goAuthor(it.author.id)">
            <Avatar :seed="it.author.id" :name="it.author.username" :size="20" />
            <span class="ellipsis">@{{ it.author.username }}</span>
          </div>
        </div>
      </div>
      <div v-else-if="!searching" class="empty dim">没有找到相关内容，换个关键词试试</div>
    </section>

    <!-- 热榜 -->
    <section v-else class="section">
      <h2 class="stitle"><DyIcon name="fire" :size="20" /> 热度榜 · 高赞视频</h2>
      <div class="grid">
        <div v-for="(it, i) in hot" :key="it.id" class="cell" @click="goVideo(it.id)">
          <div class="thumb" :style="{ backgroundImage: `url('${bg(it)}')` }">
            <span class="rank" v-if="i < 3">{{ i + 1 }}</span>
            <span class="plays"><DyIcon name="heart" :size="12" />{{ fmtCount(it.likes_count) }}</span>
          </div>
          <p class="t ellipsis">{{ it.title }}</p>
          <div class="a" @click.stop="goAuthor(it.author.id)">
            <Avatar :seed="it.author.id" :name="it.author.username" :size="20" />
            <span class="ellipsis">@{{ it.author.username }}</span>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.disc {
  height: 100%;
  overflow-y: auto;
  background: var(--bg);
  color: var(--text-1);
  padding: 0 4vw 60px;
}
.head {
  position: sticky;
  top: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 18px 0;
  background: var(--bg);
  backdrop-filter: blur(8px);
}
.back {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: var(--surface);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--shadow-1);
}
.head h1 {
  font-size: 20px;
  margin: 0;
  white-space: nowrap;
}
.search {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--radius-full);
  padding: 0 8px 0 16px;
  height: 42px;
  max-width: 480px;
  color: var(--text-3);
}
.search input {
  flex: 1;
  background: none;
  border: none;
  outline: none;
  font-size: 14px;
}
.go {
  background: var(--brand-red);
  color: #fff;
  height: 30px;
  padding: 0 16px;
  border-radius: var(--radius-full);
  font-size: 13px;
}
.pub {
  color: var(--brand-red);
  font-weight: 600;
  white-space: nowrap;
}
.section {
  margin-top: 10px;
}
.stitle {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  color: var(--brand-red);
  margin: 8px 0 16px;
}
.reset {
  color: var(--text-3);
  font-size: 13px;
  margin-left: auto;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 18px;
}
.cell {
  cursor: pointer;
  transition: transform 0.18s;
}
.cell:hover {
  transform: translateY(-4px);
}
.thumb {
  position: relative;
  aspect-ratio: 9 / 14;
  border-radius: var(--radius-md);
  background-size: cover;
  background-position: center;
  overflow: hidden;
  box-shadow: var(--shadow-1);
}
.thumb::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.45), transparent 40%);
}
.rank {
  position: absolute;
  top: 8px;
  left: 8px;
  z-index: 2;
  width: 26px;
  height: 26px;
  border-radius: 8px;
  background: var(--brand-red);
  color: #fff;
  font-weight: 800;
  font-size: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.plays {
  position: absolute;
  right: 8px;
  bottom: 8px;
  z-index: 2;
  color: #fff;
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 4px;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
}
.t {
  margin: 8px 0 4px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-1);
}
.a {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-2);
  font-size: 12px;
}
.empty {
  text-align: center;
  padding: 80px 0;
}
</style>
