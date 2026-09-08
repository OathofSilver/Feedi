<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { uploadVideoFile, uploadCoverFile, publishVideo } from '@/api/video'
import { toast, toastErr } from '@/stores/toast'
import DyIcon from '@/components/common/DyIcon.vue'

const router = useRouter()

const videoFile = ref<File | null>(null)
const coverFile = ref<File | null>(null)
const title = ref('')
const desc = ref('')
const videoPreview = ref('')
const coverPreview = ref('')
const uploading = ref(false)
const step = ref<'pick' | 'meta'>('pick')
const playUrl = ref('')
const coverUrl = ref('')

function onPickVideo(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  videoFile.value = f
  videoPreview.value = URL.createObjectURL(f)
  step.value = 'meta'
}
function onPickCover(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  coverFile.value = f
  coverPreview.value = URL.createObjectURL(f)
}

async function uploadVideoOnly(): Promise<boolean> {
  if (!videoFile.value) return false
  try {
    const res = await uploadVideoFile(videoFile.value)
    playUrl.value = res.play_url || res.url
    if (coverFile.value) {
      try {
        const c = await uploadCoverFile(coverFile.value)
        coverUrl.value = c.cover_url || c.url
      } catch {
        toast('封面上传失败，可后续重传', 'error')
      }
    }
    return true
  } catch (e) {
    toastErr(e)
    return false
  }
}

async function publishAll() {
  if (!title.value.trim() && !playUrl.value) return toast('请填写标题', 'error')
  uploading.value = true
  // 若选了本地视频但尚未上传，先上传拿 URL
  if (videoFile.value && !playUrl.value) {
    const ok = await uploadVideoOnly()
    if (!ok) {
      uploading.value = false
      return
    }
  }
  try {
    await publishVideo({
      title: title.value || '我的视频',
      description: desc.value,
      play_url: playUrl.value,
      cover_url: coverUrl.value,
    })
    toast('发布成功', 'success')
    router.replace('/feed')
  } catch (e) {
    toastErr(e)
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="up">
    <header class="head">
      <button class="back" @click="router.back()"><DyIcon name="arrowLeft" :size="22" /></button>
      <h1>发布视频</h1>
    </header>

    <div class="body">
      <template v-if="step === 'pick'">
        <label class="drop">
          <input type="file" accept="video/mp4,video/*" @change="onPickVideo" />
          <DyIcon name="plus" :size="40" />
          <p>点击选择视频文件</p>
          <span>支持 .mp4，建议竖屏 9:16，最大 200MB</span>
        </label>
      </template>

      <template v-else>
        <div class="row">
          <div class="left">
            <div v-if="videoPreview" class="vprev">
              <video :src="videoPreview" controls muted></video>
            </div>
            <label class="cov-btn">
              <input type="file" accept="image/*" @change="onPickCover" />
              选择封面
            </label>
            <div v-if="coverPreview" class="cprev">
              <img :src="coverPreview" alt="" />
            </div>
          </div>
          <div class="right">
            <label class="field">
              <span>标题</span>
              <input v-model="title" maxlength="40" placeholder="一句话描述你的视频" />
            </label>
            <label class="field">
              <span>简介（话题标签）</span>
              <textarea v-model="desc" rows="3" maxlength="120" placeholder="可加 #话题 便于被发现"></textarea>
            </label>
          </div>
        </div>

        <div class="actions">
          <button class="ghost" @click="step = 'pick'; videoFile = null">重新选择</button>
          <button class="pub" :disabled="uploading" @click="publishAll">
            {{ uploading ? '处理中…' : '发布' }}
          </button>
        </div>
        <p class="hint dim">上传与发布会写入后端；若后端未配置视频静态目录，播放链接可能不可访问（界面会优雅降级）。</p>
      </template>
    </div>
  </div>
</template>

<style scoped>
.up {
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
.body {
  padding: 10px 24px 60px;
}
.drop {
  height: 300px;
  border: 2px dashed var(--line);
  border-radius: var(--radius-lg);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  color: var(--text-2);
  transition: border 0.2s, background 0.2s;
}
.drop:hover {
  border-color: var(--brand-cyan);
  background: color-mix(in srgb, var(--brand-cyan) 6%, transparent);
}
.drop input {
  display: none;
}
.drop p {
  font-size: 16px;
  margin: 4px 0 0;
}
.drop span {
  font-size: 12px;
  color: var(--text-3);
}
.row {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}
.left {
  width: 240px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.vprev video {
  width: 100%;
  aspect-ratio: 9/16;
  background: #000;
  border-radius: var(--radius-md);
}
.cprev img {
  width: 100%;
  aspect-ratio: 9/16;
  object-fit: cover;
  border-radius: var(--radius-md);
}
.cov-btn {
  text-align: center;
  padding: 10px;
  border: 1px dashed var(--line);
  border-radius: var(--radius-md);
  color: var(--brand-red);
  cursor: pointer;
  font-size: 13px;
}
.cov-btn input {
  display: none;
}
.right {
  flex: 1;
  min-width: 260px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.field > span {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--text-2);
}
.field input,
.field textarea {
  width: 100%;
  border: 1px solid var(--line);
  background: var(--surface);
  border-radius: var(--radius-md);
  padding: 10px 14px;
  font-size: 14px;
  outline: none;
  resize: vertical;
}
.field input:focus,
.field textarea:focus {
  border-color: var(--brand-cyan);
}
.actions {
  display: flex;
  gap: 12px;
  margin-top: 26px;
}
.ghost,
.pub {
  height: 44px;
  padding: 0 28px;
  border-radius: var(--radius-full);
  font-size: 15px;
}
.ghost {
  background: var(--surface);
  border: 1px solid var(--line);
}
.pub {
  background: linear-gradient(90deg, var(--brand-red), #f04f7a);
  color: #fff;
  font-weight: 600;
}
.pub:disabled {
  opacity: 0.6;
}
.hint {
  font-size: 12px;
  margin-top: 16px;
  line-height: 1.6;
}
</style>
