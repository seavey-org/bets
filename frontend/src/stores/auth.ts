import { defineStore } from 'pinia'
import { watch } from 'vue'
import api from '../services/api'

interface User {
  id: string
  email: string
  name: string
  avatar_url: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    loading: true,
  }),

  getters: {
    isAuthenticated: (state) => !!state.user,
  },

  actions: {
    async fetchUser() {
      try {
        const { data } = await api.get('/auth/me')
        this.user = data
      } catch {
        this.user = null
      } finally {
        this.loading = false
      }
    },

    /** Returns a promise that resolves once loading becomes false. */
    untilReady() {
      if (!this.loading) return Promise.resolve()
      return new Promise<void>((resolve) => {
        const unwatch = watch(
          () => this.loading,
          (loading) => {
            if (!loading) {
              unwatch()
              resolve()
            }
          },
        )
      })
    },

    async logout() {
      try {
        await api.post('/auth/logout')
      } finally {
        this.user = null
      }
    },

    async register(email: string, password: string, name: string) {
      const { data } = await api.post('/auth/register', { email, password, name })
      this.user = data
      return data
    },

    async login(email: string, password: string) {
      const { data } = await api.post('/auth/login', { email, password })
      this.user = data
      return data
    },

    loginWithGoogle() {
      window.location.href = '/api/auth/google'
    },
  },
})
