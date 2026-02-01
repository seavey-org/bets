<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const router = useRouter()

const mode = ref<'login' | 'register'>('login')
const email = ref('')
const password = ref('')
const name = ref('')
const error = ref('')
const loading = ref(false)

async function handleSubmit() {
  error.value = ''
  loading.value = true
  try {
    if (mode.value === 'register') {
      await authStore.register(email.value, password.value, name.value)
    } else {
      await authStore.login(email.value, password.value)
    }
    router.push('/')
  } catch (e: unknown) {
    const axiosError = e as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Something went wrong'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900 -mt-6">
    <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-lg p-8 max-w-md w-full mx-4 text-center">
      <h1 class="text-4xl font-bold text-blue-600 dark:text-blue-400 mb-2">
        Bets
      </h1>
      <p class="text-gray-500 dark:text-gray-400 mb-8">
        Friendly betting pools with your crew
      </p>

      <!-- Local auth form -->
      <form
        class="space-y-4 text-left mb-6"
        @submit.prevent="handleSubmit"
      >
        <div v-if="mode === 'register'">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
          <input
            v-model="name"
            type="text"
            required
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white"
            placeholder="Your name"
          >
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Email</label>
          <input
            v-model="email"
            type="email"
            required
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white"
            placeholder="you@example.com"
          >
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
          <input
            v-model="password"
            type="password"
            required
            :minlength="mode === 'register' ? 8 : undefined"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white"
            placeholder="Password"
          >
          <p
            v-if="mode === 'register'"
            class="text-xs text-gray-400 mt-1"
          >
            Minimum 8 characters
          </p>
        </div>

        <p
          v-if="error"
          class="text-red-500 dark:text-red-400 text-sm"
        >
          {{ error }}
        </p>

        <button
          type="submit"
          :disabled="loading"
          class="w-full px-4 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium"
        >
          {{ loading ? 'Please wait...' : mode === 'register' ? 'Create Account' : 'Sign In' }}
        </button>
      </form>

      <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">
        <template v-if="mode === 'login'">
          Don't have an account?
          <button
            class="text-blue-600 dark:text-blue-400 hover:underline font-medium"
            @click="mode = 'register'; error = ''"
          >
            Sign up
          </button>
        </template>
        <template v-else>
          Already have an account?
          <button
            class="text-blue-600 dark:text-blue-400 hover:underline font-medium"
            @click="mode = 'login'; error = ''"
          >
            Sign in
          </button>
        </template>
      </p>

      <div class="relative mb-6">
        <div class="absolute inset-0 flex items-center">
          <div class="w-full border-t border-gray-300 dark:border-gray-600" />
        </div>
        <div class="relative flex justify-center text-sm">
          <span class="px-2 bg-white dark:bg-gray-800 text-gray-500 dark:text-gray-400">or</span>
        </div>
      </div>

      <button
        class="w-full flex items-center justify-center gap-3 px-6 py-3 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm hover:shadow-md transition-shadow text-gray-700 dark:text-gray-200 font-medium"
        @click="authStore.loginWithGoogle()"
      >
        <svg
          class="w-5 h-5"
          viewBox="0 0 24 24"
        >
          <path
            d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
            fill="#4285F4"
          />
          <path
            d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
            fill="#34A853"
          />
          <path
            d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
            fill="#FBBC05"
          />
          <path
            d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
            fill="#EA4335"
          />
        </svg>
        Sign in with Google
      </button>

      <p class="mt-6 text-xs text-gray-400 dark:text-gray-500">
        Create or join betting pools with friends, family, and coworkers.
      </p>
    </div>
  </div>
</template>
