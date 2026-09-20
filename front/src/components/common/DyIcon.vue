<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    name: string
    size?: number
    active?: boolean
  }>(),
  { size: 24, active: false },
)

// 线性图标路径（fill="currentColor" 的路径集，部分用空心）
const paths: Record<string, string[]> = {
  home: [
    'M12 3l9 8h-3v9h-4v-6h-4v6H6v-9H3l9-8z',
  ],
  discover: [
    'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 2a8 8 0 1 1 0 16 8 8 0 0 1 0-16zm3.5 1.5l-2.2 6.3a1 1 0 0 1-.6.6l-6.2 2.1 2.1-6.2a1 1 0 0 1 .6-.6l6.3-2.2z',
  ],
  plus: ['M11 5h2v6h6v2h-6v6h-2v-6H5v-2h6V5z'],
  user: [
    'M12 12a5 5 0 1 0-5-5 5 5 0 0 0 5 5zm0 2c-3.5 0-7 2-7 5v1h14v-1c0-3-3.5-5-7-5z',
  ],
  bell: [
    'M12 3a6 6 0 0 0-6 6v4l-2 3h16l-2-3V9a6 6 0 0 0-6-6zm-1.5 15a1.5 1.5 0 0 0 3 0h-3z',
  ],
  heart: [
    // 空心爱心：用 outline 填充规则由 CSS fill=none+stroke 处理会不统一，这里直接给可填充 path
    'M12 21s-6.7-4.3-9.3-8.2C.8 10.2 1.5 6.3 4.6 5c2-.9 4.2 0 5.4 1.9L12 9l2-2.1c1.2-1.9 3.4-2.8 5.4-1.9 3.1 1.3 3.8 5.2 1.9 7.8C18.7 16.7 12 21 12 21z',
  ],
  comment: [
    'M12 3C7 3 3 6.6 3 11c0 2.2 1 4.2 2.7 5.6-.2 1.1-.7 2-1.7 2.7-.3.2-.1.7.3.7 2.5 0 4.3-1.3 5.2-2.4.8.2 1.6.4 2.5.4 5 0 9-3.6 9-8S17 3 12 3z',
  ],
  share: [
    'M14 4l6 4-6 4v-2.5C8 10 6 12 5 16c1.5-2.5 3.5-3.5 9-3.5V14l6-6-6-6v2z',
  ],
  more: [
    'M5 10a2 2 0 1 1 0 4 2 2 0 0 1 0-4zm7 0a2 2 0 1 1 0 4 2 2 0 0 1 0-4zm7 0a2 2 0 1 1 0 4 2 2 0 0 1 0-4z',
  ],
  play: ['M8 5v14l11-7-11-7z'],
  pause: ['M7 5h3v14H7V5zm7 0h3v14h-3V5z'],
  close: ['M18.3 5.7L12 12l6.3 6.3-1.4 1.4L12 13.4l-6.3 6.3-1.4-1.4L10.6 12 4.3 5.7l1.4-1.4L12 10.6l6.3-6.3 1.4 1.4z'],
  music: [
    'M9 17.5a2.5 2.5 0 1 1-2-2.45V6l9-2.2V7L10 8.9v8.6z',
  ],
  search: [
    'M10 3a7 7 0 1 0 4.2 12.6l4.6 4.6 1.4-1.4-4.6-4.6A7 7 0 0 0 10 3zm0 2a5 5 0 1 1 0 10 5 5 0 0 1 0-10z',
  ],
  arrowLeft: ['M15.4 4.6L8.8 12l6.6 7.4-1.4 1.3L6.2 12l7.8-8.7 1.4 1.3z'],
  check: ['M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z'],
  send: ['M2 21l21-9L2 3v7l15 2-15 2v7z'],
  fire: [
    'M12 2c1 4-2 5-2 8 0 2 1.5 3 1.5 3S15 11 13 7c2 1 4 3.5 4 7a5 5 0 1 1-10 0c0-2 1-3 1-3s-1 3 0 5c-2-1-3-3-3-5 0-4.5 3-8 5-9z',
  ],
  trash: [
    'M9 3h6l1 2h4v2H4V5h4l1-2zM6 9h12l-1 12H7L6 9zm4 2v8h1v-8h-1zm3 0v8h1v-8h-1z',
  ],
}

const fullPaths = computed(() => paths[props.name] || [''])
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="currentColor"
    :class="{ 'is-heart': name === 'heart', 'is-active': active }"
    aria-hidden="true"
  >
    <path v-for="(d, i) in fullPaths" :key="i" :d="d" />
  </svg>
</template>

<style scoped>
svg {
  display: block;
  transition: transform 0.2s ease;
}
.is-heart.is-active {
  color: var(--danger);
}
</style>
