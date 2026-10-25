<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { api } from '../api'
import { state } from '../store'

const emit = defineEmits<{ (e: 'done'): void; (e: 'close'): void }>()

const username = ref('')
const password = ref('')
const loading = ref(false)
const err = ref('')
const publicMode = ref(false)
const showPwd = ref(false)
const userInput = ref<HTMLInputElement | null>(null)

onMounted(async () => {
  try {
    const me = await api.me()
    publicMode.value = !!me.publicMode
  } catch {
    /* 拿不到就当非公开模式，不给"先逛逛" */
  }
  await nextTick()
  userInput.value?.focus()
})

async function submit() {
  if (loading.value) return
  err.value = ''
  if (!username.value.trim() || !password.value) {
    err.value = '账号密码都要填呀'
    return
  }
  loading.value = true
  try {
    await api.login(username.value.trim(), password.value)
    emit('done')
  } catch (e) {
    err.value = e instanceof Error ? e.message : '登录失败'
    password.value = ''
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="login-layer fixed inset-0 z-[80] flex items-center justify-center px-4 py-10">
      <!-- 只压一层很淡的雾，背后的主页和壁纸照样看得见 -->
      <div class="login-veil absolute inset-0" />

      <form class="login-card panel-solid relative w-full max-w-[340px] rounded-3xl p-6" @submit.prevent="submit">
        <div class="mb-5 text-center">
          <img
            v-if="state.settings?.favicon"
            :src="state.settings.favicon"
            class="mx-auto mb-3 h-12 w-12 rounded-2xl object-contain"
            alt=""
          />
          <div v-else class="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-2xl bg-sky-500/20 text-2xl">🐋</div>
          <h1 class="text-base font-semibold">{{ state.settings?.title || '鲸鱼导航' }}</h1>
          <p class="mt-1 text-[11px] opacity-60">登录后就能编辑自己的导航</p>
        </div>

        <label class="label">账号</label>
        <input
          ref="userInput"
          v-model="username"
          class="field mb-3"
          autocomplete="username"
          placeholder="admin"
          spellcheck="false"
        />

        <label class="label">密码</label>
        <div class="relative mb-4">
          <input
            v-model="password"
            :type="showPwd ? 'text' : 'password'"
            class="field pr-10"
            autocomplete="current-password"
            placeholder="••••••"
          />
          <button
            type="button"
            class="absolute right-1 top-1/2 grid h-7 w-7 -translate-y-1/2 place-items-center rounded-lg opacity-50 hover:opacity-100"
            :title="showPwd ? '隐藏密码' : '显示密码'"
            @click="showPwd = !showPwd"
          >
            <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.7">
              <path v-if="!showPwd" d="M2 12s3.6-6.5 10-6.5S22 12 22 12s-3.6 6.5-10 6.5S2 12 2 12Z" stroke-linejoin="round" />
              <circle v-if="!showPwd" cx="12" cy="12" r="2.6" />
              <path v-else d="m4 4 16 16M9.9 5.7A10.6 10.6 0 0 1 12 5.5c6.4 0 10 6.5 10 6.5a17 17 0 0 1-3.4 4M6.4 7.9A17 17 0 0 0 2 12s3.6 6.5 10 6.5c1 0 1.9-.15 2.8-.42" stroke-linecap="round" />
            </svg>
          </button>
        </div>

        <p v-if="err" class="mb-3 rounded-lg bg-red-500/10 px-2.5 py-1.5 text-[11px] text-red-400">{{ err }}</p>

        <button type="submit" class="btn-primary h-10 w-full text-sm" :disabled="loading">
          {{ loading ? '登录中…' : '登录' }}
        </button>

        <button
          v-if="publicMode"
          type="button"
          class="mt-3 w-full text-center text-[11px] text-sky-600 transition hover:text-sky-500 dark:text-slate-400 dark:hover:text-slate-200"
          @click="emit('close')"
        >
          先随便逛逛 →
        </button>
        <button
          v-else
          type="button"
          class="mt-3 w-full text-center text-[11px] opacity-60 transition hover:opacity-100"
          @click="emit('close')"
        >
          关闭
        </button>
      </form>
    </div>
  </Teleport>
</template>

<style scoped>
.login-layer {
  animation: veilIn 0.25s ease both;
}
@keyframes veilIn {
  from {
    opacity: 0;
  }
}

/* 其他地方保持透明：只一点点暗，不磨砂，背景壁纸原样透出来 */
.login-veil {
  background: radial-gradient(circle at 50% 42%, rgba(2, 6, 23, 0.06), rgba(2, 6, 23, 0.34));
}
html:not(.dark) .login-veil {
  background: radial-gradient(circle at 50% 42%, rgba(255, 255, 255, 0.08), rgba(15, 23, 42, 0.18));
}

.login-card {
  animation: cardIn 0.32s cubic-bezier(0.22, 1, 0.36, 1) both;
  box-shadow: 0 24px 64px -18px rgba(0, 0, 0, 0.55);
}
@keyframes cardIn {
  from {
    opacity: 0;
    transform: translateY(14px) scale(0.97);
  }
}
</style>
