<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { uploadVideoChunked, uploadCoverFile, publishVideo } from '@/api/video'
import { toast, toastErr } from '@/stores/toast'
import DyIcon from '@/components/common/DyIcon.vue'

const router = useRouter()

const MAX_VIDEO_SIZE = 200 * 1024 * 1024 // 与界面提示一致：200MB

const videoFile = ref<File | null>(null)
const coverFile = ref<File | null>(null)
const title = ref('')
const desc = ref('')
const videoPreview = ref('')
const coverPreview = ref('')
const uploading = ref(false)
const uploadText = ref('') // 分片上传进度提示，如「上传中 12/40」
const step = ref<'pick' | 'meta'>('pick')
const playUrl = ref('')
const coverUrl = ref('')

/** 释放本地预览 blob URL，避免内存泄漏 */
function clearPreviews() {
  if (videoPreview.value) URL.revokeObjectURL(videoPreview.value)
  if (coverPreview.value) URL.revokeObjectURL(coverPreview.value)
  videoPreview.value = ''
  coverPreview.value = ''
}
onBeforeUnmount(clearPreviews)

function onPickVideo(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  if (f.size > MAX_VIDEO_SIZE) {
    toast('视频不能超过 200MB', 'error')
    return
  }
  if (videoPreview.value) URL.revokeObjectURL(videoPreview.value)
  videoFile.value = f
  videoPreview.value = URL.createObjectURL(f)
  // 重新选片：作废此前可能已上传的 URL 引用，避免发布旧文件
  playUrl.value = ''
  step.value = 'meta'
}
function onPickCover(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  if (coverPreview.value) URL.revokeObjectURL(coverPreview.value)
  coverFile.value = f
  coverPreview.value = URL.createObjectURL(f)
  coverUrl.value = ''
}

/** 回到选片步骤：清空本地文件/预览/已上传引用 */
function reselect() {
  step.value = 'pick'
  videoFile.value = null
  coverFile.value = null
  playUrl.value = ''
  coverUrl.value = ''
  clearPreviews()
}

async function uploadVideoOnly(): Promise<boolean> {
  if (!videoFile.value) return false
  try {
    // 分片上传：断点续传 + 并发；进度实时回显
    const res = await uploadVideoChunked(videoFile.value, (done, total) => {
      uploadText.value = total > 0 ? `上传中 ${done}/${total}` : '上传中…'
    })
    uploadText.value = ''
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
    uploadText.value = ''
    toastErr(e)
    return false
  }
}

async function publishAll() {
  uploading.value = true
  // 选了本地视频但尚未上传 → 先上传拿 URL；没有可发布资源则中止
  if (videoFile.value && !playUrl.value) {
    const ok = await uploadVideoOnly()
    if (!ok) {
      uploading.value = false
      return
    }
  }
  if (!playUrl.value) {
    uploading.value = false
    return toast('请先选择视频文件', 'error')
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
          <button class="ghost" :disabled="uploading" @click="reselect">重新选择</button>
          <button class="pub" :disabled="uploading" @click="publishAll">
            {{ uploading ? uploadText || '处理中…' : '发布' }}
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
