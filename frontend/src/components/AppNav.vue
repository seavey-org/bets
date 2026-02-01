<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import ThemeToggle from './ThemeToggle.vue'

const authStore = useAuthStore()
const themeStore = useThemeStore()
const router = useRouter()
const menuOpen = ref(false)

async function logout() {
  await authStore.logout()
  router.push('/login')
}

void themeStore // used in template via ThemeToggle
</script>

<template>
  <nav class="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex justify-between h-16">
        <div class="flex items-center gap-6">
          <router-link
            to="/"
            class="text-xl font-bold text-blue-600 dark:text-blue-400"
          >
            Bets
          </router-link>
          <div class="hidden sm:flex gap-4">
            <router-link
              to="/"
              class="text-gray-600 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 text-sm font-medium"
            >
              Dashboard
            </router-link>
            <router-link
              to="/groups/new"
              class="text-gray-600 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 text-sm font-medium"
            >
              New Group
            </router-link>
            <router-link
              to="/groups/join"
              class="text-gray-600 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 text-sm font-medium"
            >
              Join Group
            </router-link>
          </div>
        </div>
        <div class="flex items-center gap-4">
          <ThemeToggle />
          <div class="flex items-center gap-2">
            <img
              v-if="authStore.user?.avatar_url"
              :src="authStore.user.avatar_url"
              :alt="authStore.user.name"
              class="w-8 h-8 rounded-full"
            >
            <span class="hidden sm:inline text-sm text-gray-600 dark:text-gray-300">
              {{ authStore.user?.name }}
            </span>
          </div>
          <button
            class="text-sm text-gray-500 hover:text-red-500 dark:text-gray-400 dark:hover:text-red-400"
            @click="logout"
          >
            Logout
          </button>
          <!-- Mobile menu button -->
          <button
            class="sm:hidden p-2 text-gray-500 dark:text-gray-400"
            @click="menuOpen = !menuOpen"
          >
            <svg
              class="w-6 h-6"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 6h16M4 12h16M4 18h16"
              />
            </svg>
          </button>
        </div>
      </div>
    </div>
    <!-- Mobile menu -->
    <div
      v-if="menuOpen"
      class="sm:hidden border-t border-gray-200 dark:border-gray-700 px-4 py-2 space-y-1"
    >
      <router-link
        to="/"
        class="block py-2 text-sm text-gray-600 dark:text-gray-300"
        @click="menuOpen = false"
      >
        Dashboard
      </router-link>
      <router-link
        to="/groups/new"
        class="block py-2 text-sm text-gray-600 dark:text-gray-300"
        @click="menuOpen = false"
      >
        New Group
      </router-link>
      <router-link
        to="/groups/join"
        class="block py-2 text-sm text-gray-600 dark:text-gray-300"
        @click="menuOpen = false"
      >
        Join Group
      </router-link>
    </div>
  </nav>
</template>
