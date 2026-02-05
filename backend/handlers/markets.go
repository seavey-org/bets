package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/codyseavey/bets/middleware"
	"github.com/codyseavey/bets/services"
)

type MarketHandler struct {
	marketService *services.MarketService
	hub           *services.Hub
}

func NewMarketHandler(marketService *services.MarketService, hub *services.Hub) *MarketHandler {
	return &MarketHandler{
		marketService: marketService,
		hub:           hub,
	}
}

func (h *MarketHandler) Create(c *gin.Context) {
	var req services.CreateMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("id")
	userID := middleware.GetUserID(c)

	market, err := h.marketService.CreateMarket(groupID, userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.hub.BroadcastToGroup(groupID, services.WSEvent{
		Type:    "market_created",
		Payload: market,
	})

	c.JSON(http.StatusCreated, market)
}

func (h *MarketHandler) List(c *gin.Context) {
	groupID := c.Param("id")
	status := c.Query("status")

	markets, err := h.marketService.GetGroupMarkets(groupID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, markets)
}

func (h *MarketHandler) Get(c *gin.Context) {
	marketID := c.Param("mid")
	market, err := h.marketService.GetMarket(marketID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "market not found"})
		return
	}
	c.JSON(http.StatusOK, market)
}

func (h *MarketHandler) Buy(c *gin.Context) {
	var req services.TradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	marketID := c.Param("mid")
	userID := middleware.GetUserID(c)

	result, err := h.marketService.BuyShares(marketID, userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.hub.BroadcastToGroup(result.GroupID, services.WSEvent{
		Type: "market_trade",
		Payload: gin.H{
			"market_id": marketID,
			"trade":     result.Trade,
		},
	})

	c.JSON(http.StatusCreated, result.Trade)
}

func (h *MarketHandler) Sell(c *gin.Context) {
	var req services.TradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	marketID := c.Param("mid")
	userID := middleware.GetUserID(c)

	result, err := h.marketService.SellShares(marketID, userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.hub.BroadcastToGroup(result.GroupID, services.WSEvent{
		Type: "market_trade",
		Payload: gin.H{
			"market_id": marketID,
			"trade":     result.Trade,
		},
	})

	c.JSON(http.StatusCreated, result.Trade)
}

type ResolveMarketRequest struct {
	WinningOutcomeID string `json:"winning_outcome_id" binding:"required"`
}

func (h *MarketHandler) Resolve(c *gin.Context) {
	var req ResolveMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	marketID := c.Param("mid")
	userID := middleware.GetUserID(c)
	member := middleware.GetGroupMember(c)
	isAdmin := member != nil && member.Role == "admin"

	groupID, err := h.marketService.ResolveMarket(marketID, req.WinningOutcomeID, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	market, _ := h.marketService.GetMarket(marketID)
	h.hub.BroadcastToGroup(groupID, services.WSEvent{
		Type:    "market_resolved",
		Payload: market,
	})

	c.JSON(http.StatusOK, gin.H{"message": "market resolved"})
}

func (h *MarketHandler) Cancel(c *gin.Context) {
	marketID := c.Param("mid")
	userID := middleware.GetUserID(c)
	member := middleware.GetGroupMember(c)
	isAdmin := member != nil && member.Role == "admin"

	groupID, err := h.marketService.CancelMarket(marketID, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.hub.BroadcastToGroup(groupID, services.WSEvent{
		Type:    "market_cancelled",
		Payload: gin.H{"market_id": marketID},
	})

	c.JSON(http.StatusOK, gin.H{"message": "market cancelled"})
}

func (h *MarketHandler) Trades(c *gin.Context) {
	marketID := c.Param("mid")
	trades, err := h.marketService.GetMarketTrades(marketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, trades)
}

func (h *MarketHandler) Quote(c *gin.Context) {
	marketID := c.Param("mid")
	outcomeID := c.Query("outcome_id")
	sharesStr := c.Query("shares")
	side := c.Query("side")

	if outcomeID == "" || sharesStr == "" || side == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outcome_id, shares, and side are required"})
		return
	}

	var shares float64
	if _, err := fmt.Sscanf(sharesStr, "%f", &shares); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid shares value"})
		return
	}

	quote, err := h.marketService.GetQuote(marketID, outcomeID, shares, side)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, quote)
}

func (h *MarketHandler) Positions(c *gin.Context) {
	groupID := c.Param("id")
	userID := middleware.GetUserID(c)

	positions, err := h.marketService.GetUserPositions(groupID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, positions)
}
