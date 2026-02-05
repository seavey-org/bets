<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useMarketsStore, type Quote } from '../stores/markets'
import { useGroupsStore } from '../stores/groups'
import { useAuthStore } from '../stores/auth'
import { formatPoints, timeAgo, formatDateTime } from '../utils/format'

const route = useRoute()
const marketsStore = useMarketsStore()
const groupsStore = useGroupsStore()
const authStore = useAuthStore()

const groupId = route.params.id as string
const marketId = route.params.mid as string

const tradeOutcomeId = ref('')
const tradeShares = ref(1)
const tradeMode = ref<'buy' | 'sell'>('buy')
const resolveOutcomeId = ref('')
const tradeError = ref('')
const adminError = ref('')
const submitting = ref(false)
const showTrades = ref(false)

// Quote preview state
const quote = ref<Quote | null>(null)
const quoteLoading = ref(false)
const quoteError = ref('')
let quoteDebounceTimer: ReturnType<typeof setTimeout> | null = null

// Watch trade inputs and fetch quote with debounce
watch(
  [tradeOutcomeId, tradeShares, tradeMode],
  () => {
    // Clear previous quote
    quote.value = null
    quoteError.value = ''

    // Cancel pending request
    if (quoteDebounceTimer) {
      clearTimeout(quoteDebounceTimer)
    }

    // Don't fetch if inputs are invalid
    if (!tradeOutcomeId.value || tradeShares.value <= 0) {
      return
    }

    // Debounce the API call
    quoteDebounceTimer = setTimeout(async () => {
      quoteLoading.value = true
      try {
        quote.value = await marketsStore.getQuote(
          groupId,
          marketId,
          tradeOutcomeId.value,
          tradeShares.value,
          tradeMode.value
        )
      } catch (e: unknown) {
        const err = e as { response?: { data?: { error?: string } } }
        quoteError.value = err.response?.data?.error || 'Failed to get quote'
      } finally {
        quoteLoading.value = false
      }
    }, 300)
  },
  { immediate: true }
)

// Clean up debounce timer on unmount to prevent state updates on unmounted component
onUnmounted(() => {
  if (quoteDebounceTimer) {
    clearTimeout(quoteDebounceTimer)
  }
})

onMounted(async () => {
  // Clear stale trades from a previously viewed market
  marketsStore.trades = []
  await Promise.all([
    marketsStore.fetchMarket(groupId, marketId),
    marketsStore.fetchPositions(groupId),
    groupsStore.fetchGroup(groupId),
  ])
})

const market = computed(() => marketsStore.activeMarket)

const canManage = computed(() => {
  if (!market.value || !authStore.user) return false
  const isCreator = market.value.created_by === authStore.user.id
  const member = groupsStore.activeGroup?.members?.find(m => m.user_id === authStore.user!.id)
  const isAdmin = member?.role === 'admin'
  return isCreator || isAdmin
})

const myBalance = computed(() => {
  if (!groupsStore.activeGroup || !authStore.user) return 0
  const member = groupsStore.activeGroup.members?.find(m => m.user_id === authStore.user!.id)
  return member?.points_balance || 0
})

const myPositions = computed(() => {
  if (!authStore.user) return []
  return marketsStore.positions.filter(p => p.market_id === marketId && p.shares > 0)
})

const winningOutcome = computed(() => {
  if (!market.value?.winning_outcome_id) return null
  return market.value.outcomes?.find(o => o.id === market.value!.winning_outcome_id)
})

const isOpen = computed(() => market.value?.status === 'open')

async function executeTrade() {
  if (!tradeOutcomeId.value) {
    tradeError.value = 'Select an outcome'
    return
  }
  if (tradeShares.value <= 0) {
    tradeError.value = 'Shares must be greater than 0'
    return
  }

  submitting.value = true
  tradeError.value = ''
  try {
    if (tradeMode.value === 'buy') {
      await marketsStore.buyShares(groupId, marketId, tradeOutcomeId.value, tradeShares.value)
    } else {
      await marketsStore.sellShares(groupId, marketId, tradeOutcomeId.value, tradeShares.value)
    }
    await Promise.all([
      marketsStore.fetchPositions(groupId),
      groupsStore.fetchGroup(groupId),
    ])
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    tradeError.value = err.response?.data?.error || `Failed to ${tradeMode.value}`
  } finally {
    submitting.value = false
  }
}

async function resolveMarket() {
  if (!resolveOutcomeId.value) {
    adminError.value = 'Select the winning outcome'
    return
  }
  adminError.value = ''
  try {
    await marketsStore.resolveMarket(groupId, marketId, resolveOutcomeId.value)
    await groupsStore.fetchGroup(groupId)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    adminError.value = err.response?.data?.error || 'Failed to resolve'
  }
}

async function cancelMarket() {
  if (!confirm('Cancel this market? All positions will be refunded. This cannot be undone.')) return
  adminError.value = ''
  try {
    await marketsStore.cancelMarket(groupId, marketId)
    await groupsStore.fetchGroup(groupId)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    adminError.value = err.response?.data?.error || 'Failed to cancel'
  }
}

async function toggleTrades() {
  showTrades.value = !showTrades.value
  if (showTrades.value && marketsStore.trades.length === 0) {
    await marketsStore.fetchTrades(groupId, marketId)
  }
}
</script>

<template>
  <div
    v-if="market"
    class="max-w-2xl mx-auto"
  >
    <!-- Header -->
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
      <div class="flex items-start justify-between mb-2">
        <h1 class="text-2xl font-bold">
          {{ market.title }}
        </h1>
        <span
          class="text-xs font-medium px-2 py-1 rounded-full"
          :class="{
            'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400': market.status === 'open',
            'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-600 dark:text-yellow-400': market.status === 'closed',
            'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400': market.status === 'resolved',
            'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400': market.status === 'cancelled',
          }"
        >
          {{ market.status.charAt(0).toUpperCase() + market.status.slice(1) }}
        </span>
      </div>
      <p
        v-if="market.description"
        class="text-gray-500 dark:text-gray-400 mb-3"
      >
        {{ market.description }}
      </p>
      <div class="flex flex-wrap gap-4 text-sm text-gray-400 dark:text-gray-500">
        <span>Created by {{ market.creator?.name }}</span>
        <span>{{ timeAgo(market.created_at) }}</span>
        <span>{{ formatPoints(market.total_volume) }} pts volume</span>
        <span>{{ market.trade_count }} trades</span>
        <span v-if="market.closes_at">Closes {{ formatDateTime(market.closes_at) }}</span>
      </div>
    </div>

    <!-- Winning banner -->
    <div
      v-if="market.status === 'resolved' && winningOutcome"
      class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-xl p-4 mb-6 text-center"
    >
      <p class="text-green-700 dark:text-green-400 font-semibold">
        Winner: {{ winningOutcome.label }}
      </p>
    </div>

    <!-- Cancelled banner -->
    <div
      v-if="market.status === 'cancelled'"
      class="bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-xl p-4 mb-6 text-center"
    >
      <p class="text-gray-500 dark:text-gray-400 font-medium">
        This market was cancelled. All positions have been refunded.
      </p>
    </div>

    <!-- Outcome probabilities -->
    <div class="space-y-3 mb-6">
      <div
        v-for="outcome in market.outcomes"
        :key="outcome.id"
        class="bg-white dark:bg-gray-800 rounded-xl border p-4"
        :class="{
          'border-green-400 dark:border-green-600': market.status === 'resolved' && outcome.id === market.winning_outcome_id,
          'border-gray-200 dark:border-gray-700': outcome.id !== market.winning_outcome_id,
        }"
      >
        <div class="flex items-center justify-between mb-2">
          <span class="font-medium">{{ outcome.label }}</span>
          <span class="text-lg font-bold text-blue-600 dark:text-blue-400">{{ Math.round(outcome.price * 100) }}%</span>
        </div>
        <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5 overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-300"
            :class="market.status === 'resolved' && outcome.id === market.winning_outcome_id ? 'bg-green-500' : 'bg-blue-500 dark:bg-blue-400'"
            :style="{ width: `${Math.round(outcome.price * 100)}%` }"
          />
        </div>
      </div>
    </div>

    <!-- My positions -->
    <div
      v-if="myPositions.length > 0"
      class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-xl p-4 mb-6"
    >
      <h3 class="text-sm font-semibold text-blue-700 dark:text-blue-400 mb-2">Your Positions</h3>
      <div class="space-y-1">
        <div
          v-for="pos in myPositions"
          :key="pos.id"
          class="flex justify-between text-sm"
        >
          <span class="text-blue-600 dark:text-blue-300">{{ pos.outcome?.label }}</span>
          <span class="font-medium text-blue-700 dark:text-blue-400">{{ pos.shares.toFixed(1) }} shares</span>
        </div>
      </div>
    </div>

    <!-- Trade form (only when open) -->
    <div
      v-if="isOpen"
      class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6"
    >
      <h2 class="text-lg font-semibold mb-4">
        Trade
      </h2>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-3">
        Your balance: {{ formatPoints(myBalance) }} pts
      </p>

      <div class="space-y-3">
        <!-- Buy/Sell toggle -->
        <div class="flex rounded-lg overflow-hidden border border-gray-300 dark:border-gray-600">
          <button
            type="button"
            class="flex-1 py-2 text-sm font-medium transition-colors"
            :class="tradeMode === 'buy' ? 'bg-green-600 text-white' : 'bg-white dark:bg-gray-700 text-gray-600 dark:text-gray-300'"
            @click="tradeMode = 'buy'"
          >
            Buy
          </button>
          <button
            type="button"
            class="flex-1 py-2 text-sm font-medium transition-colors"
            :class="tradeMode === 'sell' ? 'bg-red-600 text-white' : 'bg-white dark:bg-gray-700 text-gray-600 dark:text-gray-300'"
            @click="tradeMode = 'sell'"
          >
            Sell
          </button>
        </div>

        <div>
          <label class="block text-sm font-medium mb-1">Outcome</label>
          <select
            v-model="tradeOutcomeId"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white"
          >
            <option value="">
              Select...
            </option>
            <option
              v-for="outcome in market.outcomes"
              :key="outcome.id"
              :value="outcome.id"
            >
              {{ outcome.label }} ({{ Math.round(outcome.price * 100) }}%)
            </option>
          </select>
        </div>

        <div>
          <label class="block text-sm font-medium mb-1">Shares</label>
          <input
            v-model.number="tradeShares"
            type="number"
            min="0.1"
            step="0.1"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white"
          >
        </div>

        <!-- Quote preview -->
        <div
          v-if="quoteLoading"
          class="text-sm text-gray-500 dark:text-gray-400 py-2"
        >
          Calculating...
        </div>
        <div
          v-else-if="quote"
          class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 space-y-1"
        >
          <div class="flex justify-between text-sm">
            <span class="text-gray-600 dark:text-gray-300">
              {{ tradeMode === 'buy' ? 'Cost' : 'Payout' }}:
            </span>
            <span class="font-semibold" :class="tradeMode === 'buy' ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'">
              {{ tradeMode === 'buy' ? '-' : '+' }}{{ formatPoints(tradeMode === 'buy' ? quote.cost! : quote.payout!) }} pts
            </span>
          </div>
          <div class="flex justify-between text-sm text-gray-500 dark:text-gray-400">
            <span>Avg price:</span>
            <span>{{ (quote.avg_price * 100).toFixed(1) }}% per share</span>
          </div>
          <div
            v-if="quote.new_prices.length > 0"
            class="pt-1 border-t border-gray-200 dark:border-gray-600 mt-1"
          >
            <span class="text-xs text-gray-500 dark:text-gray-400">Price impact:</span>
            <div class="flex flex-wrap gap-2 mt-1">
              <span
                v-for="np in quote.new_prices"
                :key="np.outcome_id"
                class="text-xs"
              >
                <span class="text-gray-600 dark:text-gray-300">{{ np.label }}:</span>
                <span class="text-gray-500 dark:text-gray-400">
                  {{ Math.round((market?.outcomes?.find(o => o.id === np.outcome_id)?.price ?? 0) * 100) }}%
                </span>
                <span class="text-gray-400 dark:text-gray-500">-></span>
                <span class="font-medium" :class="np.price > (market?.outcomes?.find(o => o.id === np.outcome_id)?.price ?? 0) ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                  {{ Math.round(np.price * 100) }}%
                </span>
              </span>
            </div>
          </div>
        </div>
        <p
          v-else-if="quoteError"
          class="text-amber-600 dark:text-amber-400 text-sm"
        >
          {{ quoteError }}
        </p>

        <p
          v-if="tradeError"
          class="text-red-500 dark:text-red-400 text-sm"
        >
          {{ tradeError }}
        </p>

        <button
          :disabled="submitting"
          class="w-full py-2 text-white rounded-lg font-medium disabled:opacity-50"
          :class="tradeMode === 'buy' ? 'bg-green-600 hover:bg-green-700' : 'bg-red-600 hover:bg-red-700'"
          @click="executeTrade"
        >
          {{ submitting ? 'Processing...' : (tradeMode === 'buy' ? 'Buy Shares' : 'Sell Shares') }}
        </button>
      </div>
    </div>

    <!-- Trade history toggle -->
    <div class="mb-6">
      <button
        class="text-sm text-blue-600 dark:text-blue-400 hover:underline"
        @click="toggleTrades"
      >
        {{ showTrades ? 'Hide' : 'Show' }} Trade History
      </button>
      <div
        v-if="showTrades"
        class="mt-3 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
      >
        <div
          v-if="marketsStore.trades.length === 0"
          class="p-4 text-sm text-gray-500 dark:text-gray-400 text-center"
        >
          No trades yet
        </div>
        <table
          v-else
          class="w-full text-sm"
        >
          <thead class="bg-gray-50 dark:bg-gray-700/50">
            <tr>
              <th class="text-left px-4 py-2 font-medium text-gray-500 dark:text-gray-400">User</th>
              <th class="text-left px-4 py-2 font-medium text-gray-500 dark:text-gray-400">Side</th>
              <th class="text-left px-4 py-2 font-medium text-gray-500 dark:text-gray-400">Outcome</th>
              <th class="text-right px-4 py-2 font-medium text-gray-500 dark:text-gray-400">Shares</th>
              <th class="text-right px-4 py-2 font-medium text-gray-500 dark:text-gray-400">Cost</th>
              <th class="text-right px-4 py-2 font-medium text-gray-500 dark:text-gray-400">When</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
            <tr
              v-for="trade in marketsStore.trades"
              :key="trade.id"
            >
              <td class="px-4 py-2 text-gray-700 dark:text-gray-300">{{ trade.user?.name }}</td>
              <td class="px-4 py-2">
                <span
                  class="text-xs font-medium px-1.5 py-0.5 rounded"
                  :class="trade.side === 'buy' ? 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400' : 'bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400'"
                >
                  {{ trade.side }}
                </span>
              </td>
              <td class="px-4 py-2 text-gray-700 dark:text-gray-300">{{ trade.outcome?.label }}</td>
              <td class="px-4 py-2 text-right text-gray-700 dark:text-gray-300">{{ trade.shares.toFixed(1) }}</td>
              <td class="px-4 py-2 text-right text-gray-700 dark:text-gray-300">{{ formatPoints(Math.abs(trade.points_cost)) }}</td>
              <td class="px-4 py-2 text-right text-gray-400 dark:text-gray-500">{{ timeAgo(trade.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Admin/Creator controls -->
    <div
      v-if="canManage && (market.status === 'open' || market.status === 'closed')"
      class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6"
    >
      <h2 class="text-lg font-semibold mb-4">
        Manage Market
      </h2>
      <div class="space-y-3">
        <div>
          <label class="block text-sm font-medium mb-1">Resolve: Pick the winner</label>
          <div class="flex gap-2">
            <select
              v-model="resolveOutcomeId"
              class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700"
            >
              <option value="">
                Select winning outcome...
              </option>
              <option
                v-for="outcome in market.outcomes"
                :key="outcome.id"
                :value="outcome.id"
              >
                {{ outcome.label }}
              </option>
            </select>
            <button
              class="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 font-medium"
              @click="resolveMarket"
            >
              Resolve
            </button>
          </div>
        </div>

        <p
          v-if="adminError"
          class="text-red-500 dark:text-red-400 text-sm"
        >
          {{ adminError }}
        </p>

        <button
          class="w-full py-2 bg-red-500 text-white rounded-lg hover:bg-red-600 font-medium"
          @click="cancelMarket"
        >
          Cancel Market (refund all)
        </button>
      </div>
    </div>
  </div>
</template>
