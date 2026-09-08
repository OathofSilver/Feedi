<script setup lang="ts">
import { useToastStore } from '@/stores/toast'

const store = useToastStore()
</script>

<template>
  <Teleport to="body">
    <div class="toast-host">
      <TransitionGroup name="toast">
        <div
          v-for="t in store.toasts"
          :key="t.id"
          class="toast"
          :class="`toast--${t.kind}`"
        >
          <span class="dot" />
          {{ t.text }}
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-host {
  position: fixed;
  top: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 2000;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  pointer-events: none;
}
.toast {
  background: var(--overlay);
  color: var(--text-1);
  backdrop-filter: blur(10px);
  border: 1px solid var(--line);
  padding: 10px 18px;
  border-radius: var(--radius-full);
  font-size: 14px;
  box-shadow: var(--shadow-2);
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 70vw;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: none;
}
.toast--info .dot {
  background: var(--brand-cyan);
}
.toast--success .dot {
  background: #3ddc84;
}
.toast--error .dot {
  background: var(--danger);
}
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateY(-16px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
