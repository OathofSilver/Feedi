<script setup lang="ts">
import { watch, onBeforeUnmount } from 'vue'

const props = withDefaults(
  defineProps<{
    show: boolean
    title?: string
    width?: number
    closable?: boolean
  }>(),
  { width: 420, closable: true },
)

const emit = defineEmits<{ (e: 'close'): void }>()

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.closable) emit('close')
}

// 仅展示期间注册全局 Esc 监听，避免多个常驻 Modal 叠加无效监听
watch(
  () => props.show,
  (s) => {
    if (s) window.addEventListener('keydown', onKey)
    else window.removeEventListener('keydown', onKey)
  },
  { immediate: true },
)
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="show" class="mask" @click.self="closable && emit('close')">
        <div class="panel" :style="{ width: width + 'px', maxWidth: '92vw' }">
          <header v-if="title || closable" class="head">
            <h3>{{ title }}</h3>
            <button v-if="closable" class="x" aria-label="关闭" @click="emit('close')">✕</button>
          </header>
          <div class="body">
            <slot />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  background: var(--scrim);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1500;
  backdrop-filter: blur(2px);
}
.panel {
  background: var(--bg-elev);
  border: 1px solid var(--line);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-2);
  overflow: hidden;
  max-height: 86vh;
  display: flex;
  flex-direction: column;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 8px;
}
.head h3 {
  margin: 0;
  font-size: 18px;
}
.x {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-2);
  background: var(--surface-2);
}
.body {
  padding: 8px 20px 22px;
  overflow-y: auto;
}
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.22s ease;
}
.modal-enter-active .panel,
.modal-leave-active .panel {
  transition: transform 0.22s ease, opacity 0.22s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from .panel,
.modal-leave-to .panel {
  transform: translateY(20px) scale(0.98);
}
</style>
