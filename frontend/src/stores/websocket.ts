import { defineStore } from 'pinia'
import { usePoolsStore } from './pools'
import { useGroupsStore } from './groups'
import { useMarketsStore } from './markets'

interface WSEvent {
  type: string
  payload: unknown
}

export const useWebSocketStore = defineStore('websocket', {
  state: () => ({
    socket: null as WebSocket | null,
    groupId: null as string | null,
    reconnectAttempts: 0,
    maxReconnectAttempts: 10,
    reconnectTimer: null as ReturnType<typeof setTimeout> | null,
    connected: false,
  }),

  actions: {
    connect(groupId: string) {
      this.disconnect()
      this.groupId = groupId
      this.reconnectAttempts = 0
      this.doConnect()
    },

    doConnect() {
      if (!this.groupId) return

      // The httpOnly auth cookie is sent automatically by the browser on the
      // WS upgrade HTTP request, so no need to extract it from JS.
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${protocol}//${window.location.host}/ws/groups/${this.groupId}`

      this.socket = new WebSocket(url)

      this.socket.onopen = () => {
        this.connected = true
        this.reconnectAttempts = 0
      }

      this.socket.onmessage = (event) => {
        try {
          const wsEvent: WSEvent = JSON.parse(event.data)
          this.handleEvent(wsEvent)
        } catch {
          // ignore malformed messages
        }
      }

      this.socket.onclose = () => {
        this.connected = false
        this.scheduleReconnect()
      }

      this.socket.onerror = () => {
        this.socket?.close()
      }
    },

    disconnect() {
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }
      if (this.socket) {
        this.socket.close()
        this.socket = null
      }
      this.connected = false
      this.groupId = null
    },

    scheduleReconnect() {
      if (this.reconnectAttempts >= this.maxReconnectAttempts) return
      if (!this.groupId) return

      const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000)
      this.reconnectAttempts++
      this.reconnectTimer = setTimeout(() => {
        this.doConnect()
      }, delay)
    },

    handleEvent(event: WSEvent) {
      const poolsStore = usePoolsStore()
      const groupsStore = useGroupsStore()
      const marketsStore = useMarketsStore()

      switch (event.type) {
      case 'pool_created':
      case 'pool_locked':
      case 'pool_resolved':
      case 'pool_cancelled':
      case 'bet_placed':
        if (this.groupId) {
          poolsStore.fetchPools(this.groupId)
          if (poolsStore.activePool) {
            poolsStore.fetchPool(this.groupId, poolsStore.activePool.id)
          }
        }
        break
      case 'market_created':
      case 'market_resolved':
      case 'market_cancelled':
        if (this.groupId) {
          marketsStore.fetchMarkets(this.groupId)
          if (marketsStore.activeMarket) {
            marketsStore.fetchMarket(this.groupId, marketsStore.activeMarket.id)
          }
        }
        break
      case 'market_trade':
        if (this.groupId) {
          if (marketsStore.activeMarket) {
            marketsStore.fetchMarket(this.groupId, marketsStore.activeMarket.id)
            marketsStore.fetchPositions(this.groupId)
          }
          marketsStore.fetchMarkets(this.groupId)
        }
        break
      case 'member_joined':
      case 'member_kicked':
      case 'points_granted':
        if (this.groupId) {
          groupsStore.fetchGroup(this.groupId)
        }
        break
      }
    },
  },
})
