<script setup lang="ts">
import ToastHost from '@/components/common/ToastHost.vue'
</script>

<template>
  <div class="app-shell">
    <router-view />
    <ToastHost />
  </div>
</template>

<style scoped>
.app-shell {
  height: 100%;
  width: 100%;
}
</style>
