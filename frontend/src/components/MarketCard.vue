<script setup lang="ts">
import type { Market } from '../stores/markets'
import { timeAgo } from '../utils/format'
import { computed } from 'vue'

const props = defineProps<{
  market: Market
  groupId: string
}>()

const statusConfig = computed(() => {
  const map: Record<string, { label: string; classes: string }> = {
    open: { label: 'Open', classes: 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400' },
    closed: { label: 'Closed', classes: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-600 dark:text-yellow-400' },
    resolved: { label: 'Resolved', classes: 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400' },
    cancelled: { label: 'Cancelled', classes: 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400' },
  }
  return map[props.market.status] || map.open
})

const topOutcomes = computed(() => {
  return [...(props.market.outcomes || [])].sort((a, b) => b.price - a.price).slice(0, 4)
})
</script>

<template>
  <router-link
    :to="`/groups/${groupId}/markets/${market.id}`"
    class="block bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md transition-shadow"
  >
    <div class="flex items-start justify-between mb-2">
      <h3 class="text-lg font-semibold">
        {{ market.title }}
      </h3>
      <span
        class="text-xs font-medium px-2 py-0.5 rounded-full shrink-0 ml-2"
        :class="statusConfig.classes"
      >
        {{ statusConfig.label }}
      </span>
    </div>
    <p
      v-if="market.description"
      class="text-sm text-gray-500 dark:text-gray-400 mb-3"
    >
      {{ market.description }}
    </p>

    <!-- Outcome probability bars -->
    <div
      v-if="topOutcomes.length"
      class="space-y-1.5 mb-3"
    >
      <div
        v-for="outcome in topOutcomes"
        :key="outcome.id"
        class="flex items-center gap-2"
      >
        <span class="text-xs text-gray-600 dark:text-gray-300 w-24 truncate">{{ outcome.label }}</span>
        <div class="flex-1 bg-gray-200 dark:bg-gray-700 rounded-full h-2 overflow-hidden">
          <div
            class="h-full rounded-full bg-blue-500 dark:bg-blue-400"
            :style="{ width: `${Math.round(outcome.price * 100)}%` }"
          />
        </div>
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400 w-10 text-right">{{ Math.round(outcome.price * 100) }}%</span>
      </div>
    </div>

    <div class="flex items-center gap-4 text-xs text-gray-400 dark:text-gray-500">
      <span>{{ market.trade_count }} trades</span>
      <span>{{ market.total_volume.toLocaleString() }} pts volume</span>
      <span v-if="market.closes_at">Closes {{ timeAgo(market.closes_at) }}</span>
      <span class="ml-auto">{{ timeAgo(market.created_at) }}</span>
    </div>
  </router-link>
</template>
