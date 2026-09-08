<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { toast, toastErr } from '@/stores/toast'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const isRegister = computed(() => route.name === 'register')
const username = ref('')
const password = ref('')
const confirm = ref('')
const loading = ref(false)
const showPwd = ref(false)

async function submit() {
  const u = username.value.trim()
  const p = password.value
  if (!u || !p) {
    toast('请输入用户名和密码', 'error')
    return
  }
  if (isRegister.value) {
    if (p.length < 6) return toast('密码至少 6 位', 'error')
    if (p !== confirm.value) return toast('两次密码不一致', 'error')
  }
  loading.value = true
  try {
    if (isRegister.value) await auth.doRegister(u, p)
    else await auth.doLogin(u, p)
    toast(isRegister.value ? '注册成功' : '登录成功', 'success')
    const redirect = (route.query.redirect as string) || '/feed'
    router.replace(redirect)
  } catch (e) {
    toastErr(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="blob blob-a" />
    <div class="blob blob-b" />

    <div class="card">
      <div class="brand">
        <div class="logo">
          <svg viewBox="0 0 64 64">
            <path d="M26 15v20.4a8.6 8.6 0 1 1-3.2-6.8V15h6.4z" fill="#25f4ee" />
            <path
              d="M44 28.8v5.2a8.6 8.6 0 1 1-8.6-8.6c.9 0 1.7.1 2.6.4v3.5a4.8 4.8 0 1 0 3.4 4.6V22.7c.9.8 2 1.6 3.3 2.2 1 .5 2.3.9 3.6 1-.4 1-.9 1.9-1.6 2.6z"
              fill="#fe2c55"
            />
            <path
              d="M40.6 25.5c-1.3-.6-2.4-1.4-3.3-2.2"
              fill="none"
              stroke="#fff"
              stroke-width="3.4"
              stroke-linecap="round"
            />
          </svg>
        </div>
        <h1>Feedi</h1>
      </div>

      <h2>{{ isRegister ? '创建账号' : '登录账号' }}</h2>
      <p class="sub">沉浸式短视频社区，开启你的探索</p>

      <form @submit.prevent="submit">
        <label class="field">
          <span>用户名</span>
          <input v-model.trim="username" placeholder="请输入用户名" autocomplete="username" />
        </label>
        <label class="field">
          <span>密码</span>
          <div class="pwdwrap">
            <input
              v-model="password"
              :type="showPwd ? 'text' : 'password'"
              placeholder="请输入密码"
              autocomplete="current-password"
            />
            <button type="button" class="eye" @click="showPwd = !showPwd">{{ showPwd ? '🙈' : '👁' }}</button>
          </div>
        </label>
        <label v-if="isRegister" class="field">
          <span>确认密码</span>
          <input
            v-model="confirm"
            type="password"
            placeholder="再次输入密码"
            autocomplete="new-password"
          />
        </label>

        <button class="submit" type="submit" :disabled="loading">
          {{ loading ? '请稍候…' : isRegister ? '注册并登录' : '登录' }}
        </button>
      </form>

      <p class="switch">
        <span v-if="isRegister">已有账号？</span>
        <span v-else>还没有账号？</span>
        <router-link :to="isRegister ? '/login' : '/register'">
          {{ isRegister ? '去登录' : '去注册' }}
        </router-link>
      </p>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  background:
    radial-gradient(circle at 15% 20%, color-mix(in srgb, var(--brand-cyan) 14%, transparent) 0, transparent 45%),
    radial-gradient(circle at 85% 80%, color-mix(in srgb, var(--brand-red) 16%, transparent) 0, transparent 45%),
    var(--bg);
}
.card {
  width: 380px;
  max-width: 92vw;
  background: var(--bg-elev);
  border: 1px solid var(--line);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-2);
  padding: 34px 34px 26px;
  position: relative;
  z-index: 2;
}
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}
.logo {
  width: 46px;
  height: 46px;
  border-radius: 14px;
  background: #161823;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.logo svg {
  width: 44px;
  height: 44px;
}
.brand h1 {
  font-size: 26px;
  margin: 0;
  letter-spacing: -0.5px;
}
h2 {
  margin: 0 0 4px;
  font-size: 22px;
}
.sub {
  margin: 0 0 22px;
  color: var(--text-3);
  font-size: 13px;
}
.field {
  display: block;
  margin-bottom: 16px;
}
.field > span {
  display: block;
  font-size: 13px;
  color: var(--text-2);
  margin-bottom: 6px;
}
.field input {
  width: 100%;
  height: 44px;
  border-radius: var(--radius-md);
  border: 1px solid var(--line);
  background: var(--surface);
  padding: 0 14px;
  font-size: 14px;
  outline: none;
  transition: border 0.2s;
}
.field input:focus {
  border-color: var(--brand-cyan);
}
.pwdwrap {
  position: relative;
}
.eye {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 14px;
  background: none;
}
.submit {
  width: 100%;
  height: 46px;
  margin-top: 6px;
  border-radius: var(--radius-md);
  background: linear-gradient(90deg, var(--brand-red), #f04f7a);
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  transition: opacity 0.2s, transform 0.1s;
}
.submit:hover {
  opacity: 0.92;
}
.submit:active {
  transform: scale(0.99);
}
.submit:disabled {
  opacity: 0.6;
  cursor: default;
}
.switch {
  text-align: center;
  margin: 18px 0 0;
  font-size: 14px;
  color: var(--text-3);
}
.switch a {
  color: var(--brand-red);
  font-weight: 600;
  margin-left: 6px;
}
</style>
