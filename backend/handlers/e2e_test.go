package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/codyseavey/bets/config"
	"github.com/codyseavey/bets/middleware"
	"github.com/codyseavey/bets/models"
	"github.com/codyseavey/bets/services"
)

// testEnv holds everything needed to drive e2e tests against the full HTTP stack.
type testEnv struct {
	router       *gin.Engine
	db           *gorm.DB
	authService  *services.AuthService
	groupService *services.GroupService
}

// testUser represents a registered user with their auth cookie for requests.
type testUser struct {
	ID     string
	Email  string
	Name   string
	Cookie *http.Cookie
}

// setupE2E wires up the full Gin router with real services and an in-memory SQLite DB.
// This is the same wiring as main.go but without the SPA serving and WebSocket handler.
func setupE2E(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Each test gets a unique in-memory DB so tests don't share state.
	dsn := fmt.Sprintf("file:%s?mode=memory&_foreign_keys=ON", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMember{},
		&models.Pool{},
		&models.PoolOption{},
		&models.Bet{},
		&models.PointsLog{},
		&models.Market{},
		&models.MarketOutcome{},
		&models.SharePosition{},
		&models.Trade{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	cfg := &config.Config{
		JWTSecret:          "test-secret-key-for-e2e",
		GoogleClientID:     "fake",
		GoogleClientSecret: "fake",
		BaseURL:            "http://localhost:8080",
	}

	authService := services.NewAuthService(db, cfg)
	groupService := services.NewGroupService(db)
	poolService := services.NewPoolService(db)
	marketService := services.NewMarketService(db)
	hub := services.NewHub()
	go hub.Run()

	authHandler := NewAuthHandler(authService, cfg.BaseURL)
	groupHandler := NewGroupHandler(groupService, hub)
	poolHandler := NewPoolHandler(poolService, groupService, hub)
	marketHandler := NewMarketHandler(marketService, hub)
	leaderboardHandler := NewLeaderboardHandler(db)

	r := gin.New()

	// Public auth routes
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
	}

	// Auth-required routes
	api := r.Group("/api")
	api.Use(middleware.AuthRequired(authService))
	{
		api.GET("/auth/me", authHandler.Me)
		api.POST("/groups", groupHandler.Create)
		api.GET("/groups", groupHandler.List)
		api.POST("/groups/join", groupHandler.Join)

		groupRoutes := api.Group("/groups/:id")
		groupRoutes.Use(middleware.GroupMemberRequired(db))
		{
			groupRoutes.GET("", groupHandler.Get)
			groupRoutes.GET("/leaderboard", leaderboardHandler.GetLeaderboard)

			// Pools
			groupRoutes.POST("/pools", poolHandler.Create)
			groupRoutes.GET("/pools", poolHandler.List)
			groupRoutes.GET("/pools/:pid", poolHandler.Get)
			groupRoutes.POST("/pools/:pid/bet", poolHandler.PlaceBet)
			groupRoutes.POST("/pools/:pid/lock", poolHandler.Lock)
			groupRoutes.POST("/pools/:pid/resolve", poolHandler.Resolve)
			groupRoutes.POST("/pools/:pid/cancel", poolHandler.Cancel)

			// Markets
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

			// Admin
			admin := groupRoutes.Group("")
			admin.Use(middleware.GroupAdminRequired())
			{
				admin.PUT("", groupHandler.Update)
				admin.POST("/grant", groupHandler.GrantPoints)
				admin.DELETE("/members/:uid", groupHandler.KickMember)
				admin.DELETE("", groupHandler.Delete)
			}
		}
	}

	return &testEnv{
		router:       r,
		db:           db,
		authService:  authService,
		groupService: groupService,
	}
}

// registerUser registers a new user and returns a testUser with their auth cookie.
func (e *testEnv) registerUser(t *testing.T, email, password, name string) *testUser {
	t.Helper()

	body := map[string]string{"email": email, "password": password, "name": name}
	resp := e.doRequest(t, "POST", "/api/auth/register", body, nil)

	if resp.Code != http.StatusCreated {
		t.Fatalf("register %s failed: %d - %s", email, resp.Code, resp.Body.String())
	}

	var user struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &user); err != nil {
		t.Fatalf("failed to parse register response: %v", err)
	}

	cookie := extractCookie(resp, middleware.CookieName)
	if cookie == nil {
		t.Fatalf("no auth cookie in register response for %s", email)
	}

	return &testUser{ID: user.ID, Email: user.Email, Name: user.Name, Cookie: cookie}
}

// doRequest executes an HTTP request against the test router.
// body is JSON-marshaled if non-nil. cookie is attached if non-nil.
func (e *testEnv) doRequest(t *testing.T, method, path string, body interface{}, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}

	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

// authedRequest makes an authenticated request using the given user's cookie.
func (e *testEnv) authedRequest(t *testing.T, method, path string, body interface{}, user *testUser) *httptest.ResponseRecorder {
	t.Helper()
	return e.doRequest(t, method, path, body, user.Cookie)
}

// parseJSON unmarshals the response body into the given target.
func parseJSON(t *testing.T, resp *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(resp.Body.Bytes(), target); err != nil {
		t.Fatalf("failed to parse response JSON: %v\nbody: %s", err, resp.Body.String())
	}
}

// extractCookie finds a cookie by name from a response.
func extractCookie(resp *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range resp.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}
