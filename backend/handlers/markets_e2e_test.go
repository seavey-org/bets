package handlers

import (
	"fmt"
	"math"
	"net/http"
	"testing"
)

func TestE2E_Market_FullLifecycle(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	// Create market
	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Will it rain?",
		"outcomes":  []string{"Yes", "No"},
		"liquidity": 100,
	}, alice)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create market: expected 201, got %d: %s", createResp.Code, createResp.Body.String())
	}

	var market struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Outcomes []struct {
			ID    string  `json:"id"`
			Label string  `json:"label"`
			Price float64 `json:"price"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	if market.Title != "Will it rain?" {
		t.Errorf("expected title 'Will it rain?', got '%s'", market.Title)
	}
	if market.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", market.Status)
	}
	if len(market.Outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(market.Outcomes))
	}

	// Prices should be ~0.5 each
	for _, o := range market.Outcomes {
		if math.Abs(o.Price-0.5) > 0.01 {
			t.Errorf("expected initial price ~0.5, got %f for %s", o.Price, o.Label)
		}
	}

	// Bob buys Yes shares
	buyResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID,
		"shares":     10,
	}, bob)
	if buyResp.Code != http.StatusCreated {
		t.Fatalf("buy: expected 201, got %d: %s", buyResp.Code, buyResp.Body.String())
	}

	var trade struct {
		ID         string  `json:"id"`
		Side       string  `json:"side"`
		Shares     float64 `json:"shares"`
		PointsCost int     `json:"points_cost"`
	}
	parseJSON(t, buyResp, &trade)

	if trade.Side != "buy" {
		t.Errorf("expected side 'buy', got '%s'", trade.Side)
	}
	if trade.PointsCost <= 0 {
		t.Errorf("expected positive cost, got %d", trade.PointsCost)
	}

	// Check price moved: Yes should be > 0.5 now
	getResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/markets/%s", groupID, market.ID), nil, alice)
	var updated struct {
		Outcomes []struct {
			ID    string  `json:"id"`
			Price float64 `json:"price"`
		} `json:"outcomes"`
	}
	parseJSON(t, getResp, &updated)
	if updated.Outcomes[0].Price <= 0.5 {
		t.Errorf("expected Yes price > 0.5 after buy, got %f", updated.Outcomes[0].Price)
	}

	// Bob sells some shares back
	sellResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/sell", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID,
		"shares":     5,
	}, bob)
	if sellResp.Code != http.StatusCreated {
		t.Fatalf("sell: expected 201, got %d: %s", sellResp.Code, sellResp.Body.String())
	}

	// Resolve: Yes wins
	resolveResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/resolve", groupID, market.ID), map[string]interface{}{
		"winning_outcome_id": market.Outcomes[0].ID,
	}, alice)
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d: %s", resolveResp.Code, resolveResp.Body.String())
	}

	// Verify resolved
	finalResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/markets/%s", groupID, market.ID), nil, alice)
	var finalMarket struct {
		Status           string `json:"status"`
		WinningOutcomeID string `json:"winning_outcome_id"`
	}
	parseJSON(t, finalResp, &finalMarket)
	if finalMarket.Status != "resolved" {
		t.Errorf("expected 'resolved', got '%s'", finalMarket.Status)
	}
	if finalMarket.WinningOutcomeID != market.Outcomes[0].ID {
		t.Errorf("expected winning outcome %s, got %s", market.Outcomes[0].ID, finalMarket.WinningOutcomeID)
	}
}

func TestE2E_Market_Cancel(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Cancel test",
		"outcomes":  []string{"A", "B"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Bob buys
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID,
		"shares":     5,
	}, bob)

	// Cancel
	cancelResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/cancel", groupID, market.ID), nil, alice)
	if cancelResp.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d: %s", cancelResp.Code, cancelResp.Body.String())
	}
}

func TestE2E_Market_Trades(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Trade history",
		"outcomes":  []string{"X", "Y"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Two trades
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID, "shares": 5,
	}, alice)
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[1].ID, "shares": 3,
	}, bob)

	// Check trade history
	tradesResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/markets/%s/trades", groupID, market.ID), nil, alice)
	if tradesResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", tradesResp.Code)
	}

	var trades []struct{ ID string }
	parseJSON(t, tradesResp, &trades)
	if len(trades) != 2 {
		t.Errorf("expected 2 trades, got %d", len(trades))
	}
}

func TestE2E_Market_Positions(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Positions test",
		"outcomes":  []string{"A", "B"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Bob buys
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID, "shares": 10,
	}, bob)

	// Check positions
	posResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/positions", groupID), nil, bob)
	if posResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", posResp.Code)
	}

	var positions []struct {
		Shares float64 `json:"shares"`
	}
	parseJSON(t, posResp, &positions)
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}
	if positions[0].Shares != 10 {
		t.Errorf("expected 10 shares, got %f", positions[0].Shares)
	}
}

func TestE2E_Market_NonMemberCannotTrade(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	charlie := env.registerUser(t, "charlie@test.com", "password123", "Charlie")

	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Private market",
		"outcomes":  []string{"A", "B"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Charlie (not a member) tries to buy
	buyResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID, "shares": 5,
	}, charlie)

	if buyResp.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member, got %d", buyResp.Code)
	}
}

func TestE2E_Market_ListMarkets(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title": "Market 1", "outcomes": []string{"A", "B"}, "liquidity": 50,
	}, alice)
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title": "Market 2", "outcomes": []string{"X", "Y"}, "liquidity": 50,
	}, alice)

	resp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/markets", groupID), nil, bob)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var markets []struct{ ID string }
	parseJSON(t, resp, &markets)
	if len(markets) != 2 {
		t.Errorf("expected 2 markets, got %d", len(markets))
	}
}

func TestE2E_Market_InsufficientPoints(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Expensive",
		"outcomes":  []string{"A", "B"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Bob tries to buy a huge amount (should fail, he only has 1000 pts)
	buyResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID, "shares": 9999,
	}, bob)

	if buyResp.Code == http.StatusCreated {
		t.Error("expected failure for insufficient points")
	}
}

func TestE2E_Market_ResolvePermissions(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Perm test",
		"outcomes":  []string{"A", "B"},
		"liquidity": 50,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Bob (non-creator, non-admin) tries to resolve
	resolveResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/resolve", groupID, market.ID), map[string]interface{}{
		"winning_outcome_id": market.Outcomes[0].ID,
	}, bob)

	if resolveResp.Code == http.StatusOK {
		t.Error("expected non-creator resolve to fail")
	}
}

func TestE2E_Market_Quote_Buy(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Quote test",
		"outcomes":  []string{"Yes", "No"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Get quote for buying 10 shares
	quoteResp := env.authedRequest(t, "GET",
		fmt.Sprintf("/api/groups/%s/markets/%s/quote?outcome_id=%s&shares=10&side=buy",
			groupID, market.ID, market.Outcomes[0].ID),
		nil, bob)

	if quoteResp.Code != http.StatusOK {
		t.Fatalf("quote: expected 200, got %d: %s", quoteResp.Code, quoteResp.Body.String())
	}

	var quote struct {
		Side      string  `json:"side"`
		Shares    float64 `json:"shares"`
		Cost      float64 `json:"cost"`
		Payout    float64 `json:"payout"`
		AvgPrice  float64 `json:"avg_price"`
		NewPrices []struct {
			OutcomeID string  `json:"outcome_id"`
			Label     string  `json:"label"`
			Price     float64 `json:"price"`
		} `json:"new_prices"`
	}
	parseJSON(t, quoteResp, &quote)

	if quote.Side != "buy" {
		t.Errorf("expected side 'buy', got '%s'", quote.Side)
	}
	if quote.Shares != 10 {
		t.Errorf("expected shares 10, got %f", quote.Shares)
	}
	if quote.Cost <= 0 {
		t.Errorf("expected positive cost, got %f", quote.Cost)
	}
	if quote.AvgPrice <= 0 || quote.AvgPrice > 1 {
		t.Errorf("expected avg_price between 0 and 1, got %f", quote.AvgPrice)
	}
	if len(quote.NewPrices) != 2 {
		t.Fatalf("expected 2 new_prices, got %d", len(quote.NewPrices))
	}

	// Sum of new prices should be ~1
	sum := 0.0
	for _, np := range quote.NewPrices {
		sum += np.Price
	}
	if math.Abs(sum-1) > 0.01 {
		t.Errorf("expected new prices to sum to 1, got %f", sum)
	}
}

func TestE2E_Market_Quote_Sell(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Quote sell test",
		"outcomes":  []string{"Yes", "No"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Bob buys first so he has shares to sell
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets/%s/buy", groupID, market.ID), map[string]interface{}{
		"outcome_id": market.Outcomes[0].ID, "shares": 10,
	}, bob)

	// Get quote for selling 5 shares
	quoteResp := env.authedRequest(t, "GET",
		fmt.Sprintf("/api/groups/%s/markets/%s/quote?outcome_id=%s&shares=5&side=sell",
			groupID, market.ID, market.Outcomes[0].ID),
		nil, bob)

	if quoteResp.Code != http.StatusOK {
		t.Fatalf("quote sell: expected 200, got %d: %s", quoteResp.Code, quoteResp.Body.String())
	}

	var quote struct {
		Side   string  `json:"side"`
		Payout float64 `json:"payout"`
	}
	parseJSON(t, quoteResp, &quote)

	if quote.Side != "sell" {
		t.Errorf("expected side 'sell', got '%s'", quote.Side)
	}
	if quote.Payout <= 0 {
		t.Errorf("expected positive payout, got %f", quote.Payout)
	}
}

func TestE2E_Market_Quote_RequiresMembership(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	charlie := env.registerUser(t, "charlie@test.com", "password123", "Charlie")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Private quote",
		"outcomes":  []string{"A", "B"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Charlie (not a member) tries to get a quote
	quoteResp := env.authedRequest(t, "GET",
		fmt.Sprintf("/api/groups/%s/markets/%s/quote?outcome_id=%s&shares=10&side=buy",
			groupID, market.ID, market.Outcomes[0].ID),
		nil, charlie)

	if quoteResp.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member quote, got %d", quoteResp.Code)
	}
}

func TestE2E_Market_Quote_InvalidParams(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/markets", groupID), map[string]interface{}{
		"title":     "Invalid params",
		"outcomes":  []string{"A", "B"},
		"liquidity": 100,
	}, alice)
	var market struct {
		ID       string `json:"id"`
		Outcomes []struct {
			ID string `json:"id"`
		} `json:"outcomes"`
	}
	parseJSON(t, createResp, &market)

	// Missing outcome_id
	resp1 := env.authedRequest(t, "GET",
		fmt.Sprintf("/api/groups/%s/markets/%s/quote?shares=10&side=buy", groupID, market.ID),
		nil, bob)
	if resp1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing outcome_id, got %d", resp1.Code)
	}

	// Missing shares
	resp2 := env.authedRequest(t, "GET",
		fmt.Sprintf("/api/groups/%s/markets/%s/quote?outcome_id=%s&side=buy", groupID, market.ID, market.Outcomes[0].ID),
		nil, bob)
	if resp2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing shares, got %d", resp2.Code)
	}

	// Missing side
	resp3 := env.authedRequest(t, "GET",
		fmt.Sprintf("/api/groups/%s/markets/%s/quote?outcome_id=%s&shares=10", groupID, market.ID, market.Outcomes[0].ID),
		nil, bob)
	if resp3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing side, got %d", resp3.Code)
	}

	// Invalid side
	resp4 := env.authedRequest(t, "GET",
		fmt.Sprintf("/api/groups/%s/markets/%s/quote?outcome_id=%s&shares=10&side=invalid", groupID, market.ID, market.Outcomes[0].ID),
		nil, bob)
	if resp4.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid side, got %d", resp4.Code)
	}
}
