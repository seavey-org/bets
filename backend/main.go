package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/codyseavey/bets/config"
	"github.com/codyseavey/bets/handlers"
	"github.com/codyseavey/bets/middleware"
	"github.com/codyseavey/bets/services"
	"github.com/codyseavey/bets/storage"
)

func main() {
	cfg := config.Load()
	db := storage.InitDB(cfg.DBPath)

	// Services
	authService := services.NewAuthService(db, cfg)
	groupService := services.NewGroupService(db)
	poolService := services.NewPoolService(db)
	marketService := services.NewMarketService(db)
	hub := services.NewHub()
	go hub.Run()

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, cfg.BaseURL)
	groupHandler := handlers.NewGroupHandler(groupService, hub)
	poolHandler := handlers.NewPoolHandler(poolService, groupService, hub)
	marketHandler := handlers.NewMarketHandler(marketService, hub)
	leaderboardHandler := handlers.NewLeaderboardHandler(db)
	wsHandler := handlers.NewWebSocketHandler(hub, authService, db)

	r := gin.Default()

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.BaseURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Auth routes (public)
	auth := r.Group("/api/auth")
	{
		auth.GET("/google", authHandler.GoogleLogin)
		auth.GET("/google/callback", authHandler.GoogleCallback)
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
	}

	// Auth-required routes
	api := r.Group("/api")
	api.Use(middleware.AuthRequired(authService))
	{
		api.GET("/auth/me", authHandler.Me)

		// Groups
		api.POST("/groups", groupHandler.Create)
		api.GET("/groups", groupHandler.List)
		api.POST("/groups/join", groupHandler.Join)

		// Group-specific (requires membership)
		groupRoutes := api.Group("/groups/:id")
		groupRoutes.Use(middleware.GroupMemberRequired(db))
		{
			groupRoutes.GET("", groupHandler.Get)
			groupRoutes.GET("/leaderboard", leaderboardHandler.GetLeaderboard)
			groupRoutes.GET("/history", leaderboardHandler.GetHistory)
			groupRoutes.GET("/stats", leaderboardHandler.GetStats)

			// Pools
			groupRoutes.POST("/pools", poolHandler.Create)
			groupRoutes.GET("/pools", poolHandler.List)
			groupRoutes.GET("/pools/:pid", poolHandler.Get)
			groupRoutes.POST("/pools/:pid/bet", poolHandler.PlaceBet)
			groupRoutes.POST("/pools/:pid/lock", poolHandler.Lock)
			groupRoutes.POST("/pools/:pid/resolve", poolHandler.Resolve)
			groupRoutes.POST("/pools/:pid/cancel", poolHandler.Cancel)

			// Markets (prediction markets)
			groupRoutes.POST("/markets", marketHandler.Create)
			groupRoutes.GET("/markets", marketHandler.List)
			groupRoutes.GET("/markets/:mid", marketHandler.Get)
			groupRoutes.POST("/markets/:mid/buy", marketHandler.Buy)
			groupRoutes.POST("/markets/:mid/sell", marketHandler.Sell)
			groupRoutes.POST("/markets/:mid/resolve", marketHandler.Resolve)
			groupRoutes.POST("/markets/:mid/cancel", marketHandler.Cancel)
			groupRoutes.GET("/markets/:mid/trades", marketHandler.Trades)
			groupRoutes.GET("/markets/:mid/quote", marketHandler.Quote)
			groupRoutes.GET("/positions", marketHandler.Positions)

			// Admin-only
			admin := groupRoutes.Group("")
			admin.Use(middleware.GroupAdminRequired())
			{
				admin.PUT("", groupHandler.Update)
				admin.POST("/grant", groupHandler.GrantPoints)
				admin.DELETE("/members/:uid", groupHandler.KickMember)
				admin.POST("/regenerate-invite", groupHandler.RegenerateInvite)
				admin.DELETE("", groupHandler.Delete)
			}
		}
	}

	// WebSocket
	r.GET("/ws/groups/:id", wsHandler.HandleConnection)

	// Serve frontend SPA
	distPath := cfg.FrontendDistPath
	if _, err := os.Stat(distPath); err == nil {
		// Serve static assets with CORS and cache headers so browsers
		// behind CDNs (Cloudflare) don't choke on cross-origin checks.
		r.Group("/assets", func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Next()
		}).Static("/", filepath.Join(distPath, "assets"))
		r.StaticFile("/favicon.ico", filepath.Join(distPath, "favicon.ico"))

		// SPA fallback: serve index.html for all unmatched routes
		r.NoRoute(func(c *gin.Context) {
			c.File(filepath.Join(distPath, "index.html"))
		})
	} else {
		log.Printf("Frontend dist not found at %s, serving API only", distPath)
	}

	log.Printf("Starting server on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
