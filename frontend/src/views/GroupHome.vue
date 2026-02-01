<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useGroupsStore } from '../stores/groups'
import { usePoolsStore } from '../stores/pools'
import { useMarketsStore } from '../stores/markets'
import { useWebSocketStore } from '../stores/websocket'
import { useAuthStore } from '../stores/auth'
import PoolCard from '../components/PoolCard.vue'
import MarketCard from '../components/MarketCard.vue'

const route = useRoute()
const groupsStore = useGroupsStore()
const poolsStore = usePoolsStore()
const marketsStore = useMarketsStore()
const wsStore = useWebSocketStore()
const authStore = useAuthStore()

const groupId = computed(() => route.params.id as string)

const isAdmin = computed(() => {
  if (!groupsStore.activeGroup || !authStore.user) return false
  const member = groupsStore.activeGroup.members?.find(m => m.user_id === authStore.user!.id)
  return member?.role === 'admin'
})

const myBalance = computed(() => {
  if (!groupsStore.activeGroup || !authStore.user) return 0
  const member = groupsStore.activeGroup.members?.find(m => m.user_id === authStore.user!.id)
  return member?.points_balance || 0
})

onMounted(async () => {
  await Promise.all([
    groupsStore.fetchGroup(groupId.value),
    poolsStore.fetchPools(groupId.value),
    marketsStore.fetchMarkets(groupId.value),
  ])
  wsStore.connect(groupId.value)
})

onUnmounted(() => {
  wsStore.disconnect()
})
</script>

<template>
  <div v-if="groupsStore.activeGroup">
    <!-- Group Header -->
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold">
            {{ groupsStore.activeGroup.name }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
            {{ groupsStore.activeGroup.members?.length || 0 }} members
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <div class="px-4 py-2 bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 rounded-lg font-semibold">
            {{ myBalance.toLocaleString() }} pts
          </div>
          <router-link
            :to="`/groups/${groupId}/pools/new`"
            class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 text-sm font-medium"
          >
            New Pool
          </router-link>
          <router-link
            :to="`/groups/${groupId}/markets/new`"
            class="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 text-sm font-medium"
          >
            New Market
          </router-link>
          <router-link
            :to="`/groups/${groupId}/leaderboard`"
            class="px-4 py-2 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-200 rounded-lg text-sm"
          >
            Leaderboard
          </router-link>
          <router-link
            :to="`/groups/${groupId}/history`"
            class="px-4 py-2 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-200 rounded-lg text-sm"
          >
            History
          </router-link>
          <router-link
            v-if="isAdmin"
            :to="`/groups/${groupId}/settings`"
            class="px-4 py-2 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-200 rounded-lg text-sm"
          >
            Settings
          </router-link>
        </div>
      </div>
    </div>

    <!-- Markets -->
    <div class="mb-8">
      <h2 class="text-lg font-semibold mb-3">
        Prediction Markets
      </h2>
      <div
        v-if="marketsStore.loading"
        class="text-center py-8 text-gray-500 dark:text-gray-400"
      >
        Loading markets...
      </div>
      <div
        v-else-if="marketsStore.markets.length === 0"
        class="text-center py-8 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700"
      >
        <p class="text-gray-500 dark:text-gray-400 mb-4">
          No markets yet. Create the first one!
        </p>
        <router-link
          :to="`/groups/${groupId}/markets/new`"
          class="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700"
        >
          Create Market
        </router-link>
      </div>
      <div
        v-else
        class="space-y-4"
      >
        <MarketCard
          v-for="market in marketsStore.markets"
          :key="market.id"
          :market="market"
          :group-id="groupId"
        />
      </div>
    </div>

    <!-- Pools -->
    <div>
      <h2 class="text-lg font-semibold mb-3">
        Betting Pools
      </h2>
      <div
        v-if="poolsStore.loading"
        class="text-center py-8 text-gray-500 dark:text-gray-400"
      >
        Loading pools...
      </div>

      <div
        v-else-if="poolsStore.pools.length === 0"
        class="text-center py-12 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700"
      >
        <p class="text-gray-500 dark:text-gray-400 mb-4">
          No pools yet. Create the first one!
        </p>
        <router-link
          :to="`/groups/${groupId}/pools/new`"
          class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
        >
          Create Pool
        </router-link>
      </div>

      <div
        v-else
        class="space-y-4"
      >
        <PoolCard
          v-for="pool in poolsStore.pools"
          :key="pool.id"
          :pool="pool"
          :group-id="groupId"
        />
      </div>
    </div>
  </div>
</template>
