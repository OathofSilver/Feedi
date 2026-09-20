import { ref, onMounted, onBeforeUnmount } from 'vue'
import type { Ref } from 'vue'

export interface PlayerHandle {
  play: () => Promise<void>
  pause: () => void
  muted: boolean
}

export interface UsePlayerOptions {
  /** 进入可视区时是否自动播放 */
  autoplay?: boolean
  /** 出错回调 */
  onError?: () => void
}

export interface PlayerController {
  videoRef: Ref<HTMLVideoElement | null>
  isPlaying: Ref<boolean>
  hasError: Ref<boolean>
  isMuted: Ref<boolean>
  play: () => Promise<void>
  pause: () => void
  toggle: () => Promise<void>
  toggleMute: () => void
  setMuted: (m: boolean) => void
}

/**
 * 视频播放控制器。暴露 videoRef 供模板绑定。
 * 自动播放策略：默认静音起播，进入视口后若用户已开声则带声。
 */
export function useVideoPlayer(opts: UsePlayerOptions = {}): PlayerController {
  const videoRef = ref<HTMLVideoElement | null>(null)
  const isPlaying = ref(false)
  const hasError = ref(false)
  const isMuted = ref(true) // 默认静音（满足浏览器自动播放策略）

  function syncState() {
    const v = videoRef.value
    if (!v) return
    isPlaying.value = !v.paused && !v.ended
    v.muted = isMuted.value
  }

  async function play() {
    const v = videoRef.value
    if (!v) return
    try {
      v.muted = isMuted.value
      await v.play()
      isPlaying.value = true
    } catch {
      // 自动播放被拒：强制静音再试一次
      try {
        isMuted.value = true
        v.muted = true
        await v.play()
        isPlaying.value = true
      } catch {
        /* ignore */
      }
    }
  }

  function pause() {
    videoRef.value?.pause()
    isPlaying.value = false
  }

  async function toggle() {
    if (isPlaying.value) pause()
    else await play()
  }

  function toggleMute() {
    isMuted.value = !isMuted.value
    if (videoRef.value) videoRef.value.muted = isMuted.value
    // 取消静音后若未播放，尝试播放
    if (!isMuted.value && !isPlaying.value) play()
  }

  function setMuted(m: boolean) {
    isMuted.value = m
    if (videoRef.value) videoRef.value.muted = m
  }

  function onError() {
    hasError.value = true
    isPlaying.value = false
    opts.onError?.()
  }

  // 具名事件处理器：保证 add/remove 引用一致，卸载时能真正移除
  function onVideoPlay() {
    isPlaying.value = true
  }
  function onVideoPause() {
    isPlaying.value = false
  }
  function onVideoTimeupdate() {
    syncState()
  }

  onMounted(() => {
    const v = videoRef.value
    if (v) {
      v.muted = isMuted.value
      v.addEventListener('error', onError)
      v.addEventListener('play', onVideoPlay)
      v.addEventListener('pause', onVideoPause)
      v.addEventListener('timeupdate', onVideoTimeupdate)
    }
  })

  onBeforeUnmount(() => {
    const v = videoRef.value
    if (v) {
      v.removeEventListener('error', onError)
      v.removeEventListener('play', onVideoPlay)
      v.removeEventListener('pause', onVideoPause)
      v.removeEventListener('timeupdate', onVideoTimeupdate)
      v.pause()
    }
  })

  return { videoRef, isPlaying, hasError, isMuted, play, pause, toggle, toggleMute, setMuted }
}
