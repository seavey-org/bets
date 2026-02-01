import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('./views/Login.vue'),
    meta: { guest: true },
  },
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('./views/Dashboard.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/new',
    name: 'GroupCreate',
    component: () => import('./views/GroupCreate.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/join',
    name: 'GroupJoin',
    component: () => import('./views/GroupJoin.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/join/:code',
    name: 'GroupJoinCode',
    component: () => import('./views/GroupJoin.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id',
    name: 'GroupHome',
    component: () => import('./views/GroupHome.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id/settings',
    name: 'GroupSettings',
    component: () => import('./views/GroupSettings.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id/pools/new',
    name: 'PoolCreate',
    component: () => import('./views/PoolCreate.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id/pools/:pid',
    name: 'PoolDetail',
    component: () => import('./views/PoolDetail.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id/markets/new',
    name: 'MarketCreate',
    component: () => import('./views/MarketCreate.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id/markets/:mid',
    name: 'MarketDetail',
    component: () => import('./views/MarketDetail.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id/leaderboard',
    name: 'Leaderboard',
    component: () => import('./views/Leaderboard.vue'),
    meta: { auth: true },
  },
  {
    path: '/groups/:id/history',
    name: 'History',
    component: () => import('./views/History.vue'),
    meta: { auth: true },
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore()

  if (to.meta.auth && !authStore.isAuthenticated) {
    next('/login')
  } else if (to.meta.guest && authStore.isAuthenticated) {
    next('/')
  } else {
    next()
  }
})
