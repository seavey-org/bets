<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMarketsStore } from '../stores/markets'

const route = useRoute()
const router = useRouter()
const marketsStore = useMarketsStore()

const groupId = route.params.id as string
const title = ref('')
const description = ref('')
const outcomes = ref(['', ''])
const closesAt = ref('')
const liquidity = ref(100)
const error = ref('')
const submitting = ref(false)

function addOutcome() {
  outcomes.value.push('')
}

function removeOutcome(index: number) {
  if (outcomes.value.length > 2) {
    outcomes.value.splice(index, 1)
  }
}

async function submit() {
  const trimmedTitle = title.value.trim()
  const trimmedOutcomes = outcomes.value.map(o => o.trim()).filter(o => o)

  if (!trimmedTitle) {
    error.value = 'Title is required'
    return
  }
  if (trimmedOutcomes.length < 2) {
    error.value = 'At least 2 outcomes are required'
    return
  }
  if (liquidity.value < 1) {
    error.value = 'Liquidity must be at least 1'
    return
  }

  submitting.value = true
  error.value = ''
  try {
    const market = await marketsStore.createMarket(
      groupId,
      trimmedTitle,
      description.value.trim(),
      trimmedOutcomes,
      closesAt.value || undefined,
      liquidity.value,
    )
    router.push(`/groups/${groupId}/markets/${market.id}`)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    error.value = err.response?.data?.error || 'Failed to create market'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="max-w-lg mx-auto">
    <h1 class="text-2xl font-bold mb-6">
      Create a Market
    </h1>

    <form
      class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 space-y-4"
      @submit.prevent="submit"
    >
      <div>
        <label class="block text-sm font-medium mb-1">Title</label>
        <input
          v-model="title"
          type="text"
          placeholder="e.g. Will it snow this weekend?"
          class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        >
      </div>

      <div>
        <label class="block text-sm font-medium mb-1">Description (optional)</label>
        <textarea
          v-model="description"
          rows="2"
          placeholder="Additional context or rules"
          class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
        />
      </div>

      <div>
        <label class="block text-sm font-medium mb-2">Outcomes</label>
        <div class="space-y-2">
          <div
            v-for="(_, index) in outcomes"
            :key="index"
            class="flex gap-2"
          >
            <input
              v-model="outcomes[index]"
              type="text"
              :placeholder="`Outcome ${index + 1}`"
              class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            >
            <button
              v-if="outcomes.length > 2"
              type="button"
              class="px-3 py-2 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg"
              @click="removeOutcome(index)"
            >
              X
            </button>
          </div>
        </div>
        <button
          type="button"
          class="mt-2 text-sm text-blue-600 dark:text-blue-400 hover:underline"
          @click="addOutcome"
        >
          + Add Outcome
        </button>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1">Liquidity (points)</label>
          <input
            v-model.number="liquidity"
            type="number"
            min="1"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          >
          <p class="text-xs text-gray-400 mt-1">Deducted from your balance to seed the market</p>
        </div>

        <div>
          <label class="block text-sm font-medium mb-1">Closes at (optional)</label>
          <input
            v-model="closesAt"
            type="datetime-local"
            class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-800 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          >
        </div>
      </div>

      <p
        v-if="error"
        class="text-red-500 dark:text-red-400 text-sm"
      >
        {{ error }}
      </p>

      <button
        type="submit"
        :disabled="submitting"
        class="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium"
      >
        {{ submitting ? 'Creating...' : 'Create Market' }}
      </button>
    </form>
  </div>
</template>
