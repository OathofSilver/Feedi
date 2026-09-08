<script setup lang="ts">
import { computed, ref } from 'vue'
import { demoGradient } from '@/api/normalize'

const props = withDefaults(
  defineProps<{
    src?: string
    name?: string
    size?: number
    seed?: number
  }>(),
  { size: 40 },
)

const imgFailed = ref(false)
const fallbackBg = computed(() => demoGradient(props.seed ?? Math.abs((props.name || '').length) + 1))
const initial = computed(() => (props.name || 'U').slice(0, 1).toUpperCase())

function onImgError() {
  imgFailed.value = true
}
</script>

<template>
  <span
    class="avatar"
    :style="{
      width: size + 'px',
      height: size + 'px',
      background: src && !imgFailed ? undefined : fallbackBg,
    }"
  >
    <img
      v-if="src && !imgFailed"
      :src="src"
      :style="{ width: size + 'px', height: size + 'px' }"
      alt=""
      @error="onImgError"
    />
    <span v-else class="initial" :style="{ fontSize: size * 0.42 + 'px' }">{{ initial }}</span>
  </span>
</template>

<style scoped>
.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  overflow: hidden;
  flex: none;
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.2);
}
.avatar img {
  object-fit: cover;
  display: block;
}
.initial {
  font-weight: 600;
}
</style>
