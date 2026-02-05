import { defineStore } from 'pinia'
import api from '../services/api'

export interface MarketOutcome {
  id: string
  market_id: string
  label: string
  shares: number
  price: number
}

export interface Market {
  id: string
  group_id: string
  title: string
  description: string
  status: 'open' | 'closed' | 'resolved' | 'cancelled'
  created_by: string
  liquidity: number
  winning_outcome_id: string
  resolved_at: string | null
  closes_at: string | null
  created_at: string
  creator: { id: string; name: string; avatar_url: string }
  outcomes: MarketOutcome[]
  total_volume: number
  trade_count: number
}

export interface Trade {
  id: string
  market_id: string
  user_id: string
  outcome_id: string
  side: 'buy' | 'sell'
  shares: number
  points_cost: number
  price: number
  created_at: string
  user: { id: string; name: string; avatar_url: string }
  outcome: MarketOutcome
}

export interface SharePosition {
  id: string
  market_id: string
  user_id: string
  outcome_id: string
  shares: number
  user: { id: string; name: string; avatar_url: string }
  outcome: MarketOutcome
}

export interface OutcomePrice {
  outcome_id: string
  label: string
  price: number
}

export interface Quote {
  side: 'buy' | 'sell'
  shares: number
  cost?: number
  payout?: number
  avg_price: number
  new_prices: OutcomePrice[]
}

export const useMarketsStore = defineStore('markets', {
  state: () => ({
    markets: [] as Market[],
    activeMarket: null as Market | null,
    trades: [] as Trade[],
    positions: [] as SharePosition[],
    loading: false,
  }),

  actions: {
    async fetchMarkets(groupId: string, status?: string) {
      this.loading = true
      try {
        const params = status ? { status } : {}
        const { data } = await api.get(`/groups/${groupId}/markets`, { params })
        this.markets = data
      } finally {
        this.loading = false
      }
    },

    async fetchMarket(groupId: string, marketId: string) {
      const { data } = await api.get(`/groups/${groupId}/markets/${marketId}`)
      this.activeMarket = data
      return data
    },

    async createMarket(groupId: string, title: string, description: string, outcomes: string[], closesAt?: string, liquidity?: number) {
      const body: Record<string, unknown> = { title, description, outcomes }
      if (closesAt) body.closes_at = new Date(closesAt).toISOString()
      if (liquidity != null) body.liquidity = liquidity
      const { data } = await api.post(`/groups/${groupId}/markets`, body)
      this.markets.unshift(data)
      return data
    },

    async buyShares(groupId: string, marketId: string, outcomeId: string, shares: number) {
      const { data } = await api.post(`/groups/${groupId}/markets/${marketId}/buy`, {
        outcome_id: outcomeId,
        shares,
      })
      await this.fetchMarket(groupId, marketId)
      return data as Trade
    },

    async sellShares(groupId: string, marketId: string, outcomeId: string, shares: number) {
      const { data } = await api.post(`/groups/${groupId}/markets/${marketId}/sell`, {
        outcome_id: outcomeId,
        shares,
      })
      await this.fetchMarket(groupId, marketId)
      return data as Trade
    },

    async resolveMarket(groupId: string, marketId: string, winningOutcomeId: string) {
      await api.post(`/groups/${groupId}/markets/${marketId}/resolve`, {
        winning_outcome_id: winningOutcomeId,
      })
      await this.fetchMarket(groupId, marketId)
    },

    async cancelMarket(groupId: string, marketId: string) {
      await api.post(`/groups/${groupId}/markets/${marketId}/cancel`)
      await this.fetchMarket(groupId, marketId)
    },

    async fetchTrades(groupId: string, marketId: string) {
      const { data } = await api.get(`/groups/${groupId}/markets/${marketId}/trades`)
      this.trades = data
      return data
    },

    async fetchPositions(groupId: string) {
      const { data } = await api.get(`/groups/${groupId}/positions`)
      this.positions = data
      return data
    },

    async getQuote(groupId: string, marketId: string, outcomeId: string, shares: number, side: 'buy' | 'sell'): Promise<Quote> {
      const { data } = await api.get(`/groups/${groupId}/markets/${marketId}/quote`, {
        params: { outcome_id: outcomeId, shares, side },
      })
      return data as Quote
    },
  },
})
