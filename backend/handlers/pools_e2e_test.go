package handlers

import (
	"fmt"
	"net/http"
	"testing"
)

// createGroupWithMembers is a helper that creates a group and has bob join it.
// Returns the group ID.
func createGroupWithMembers(t *testing.T, env *testEnv, alice, bob *testUser) string {
	t.Helper()

	createResp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Pool Test Group", "default_points": 1000,
	}, alice)
	var group struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
	}
	parseJSON(t, createResp, &group)

	env.authedRequest(t, "POST", "/api/groups/join", map[string]string{
		"invite_code": group.InviteCode,
	}, bob)

	return group.ID
}

func TestE2E_Pool_FullLifecycle(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	// Create pool
	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools", groupID), map[string]interface{}{
		"title":   "Who wins?",
		"options": []string{"Alice", "Bob"},
	}, alice)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create pool: expected 201, got %d: %s", createResp.Code, createResp.Body.String())
	}

	var pool struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Status  string `json:"status"`
		Options []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"options"`
	}
	parseJSON(t, createResp, &pool)

	if pool.Title != "Who wins?" {
		t.Errorf("expected title 'Who wins?', got '%s'", pool.Title)
	}
	if pool.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", pool.Status)
	}
	if len(pool.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(pool.Options))
	}

	// Alice bets on option 0
	betResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/bet", groupID, pool.ID), map[string]interface{}{
		"option_id": pool.Options[0].ID,
		"points":    50,
	}, alice)
	if betResp.Code != http.StatusCreated {
		t.Fatalf("alice bet: expected 201, got %d: %s", betResp.Code, betResp.Body.String())
	}

	// Bob bets on option 1
	betResp2 := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/bet", groupID, pool.ID), map[string]interface{}{
		"option_id": pool.Options[1].ID,
		"points":    30,
	}, bob)
	if betResp2.Code != http.StatusCreated {
		t.Fatalf("bob bet: expected 201, got %d: %s", betResp2.Code, betResp2.Body.String())
	}

	// Lock the pool (admin only)
	lockResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/lock", groupID, pool.ID), nil, alice)
	if lockResp.Code != http.StatusOK {
		t.Fatalf("lock: expected 200, got %d: %s", lockResp.Code, lockResp.Body.String())
	}

	// Resolve: option 0 wins (Alice's pick)
	resolveResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/resolve", groupID, pool.ID), map[string]interface{}{
		"winning_option_id": pool.Options[0].ID, // ResolveRequest

	}, alice)
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d: %s", resolveResp.Code, resolveResp.Body.String())
	}

	// Verify pool is resolved
	getResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/pools/%s", groupID, pool.ID), nil, alice)
	var resolved struct {
		Status string `json:"status"`
	}
	parseJSON(t, getResp, &resolved)
	if resolved.Status != "resolved" {
		t.Errorf("expected status 'resolved', got '%s'", resolved.Status)
	}
}

func TestE2E_Pool_BetOnLockedPool(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools", groupID), map[string]interface{}{
		"title":   "Locked pool",
		"options": []string{"A", "B"},
	}, alice)
	var pool struct {
		ID      string `json:"id"`
		Options []struct {
			ID string `json:"id"`
		} `json:"options"`
	}
	parseJSON(t, createResp, &pool)

	// Lock it
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/lock", groupID, pool.ID), nil, alice)

	// Try to bet - should fail
	betResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/bet", groupID, pool.ID), map[string]interface{}{
		"option_id": pool.Options[0].ID,
		"points":    10,
	}, bob)
	if betResp.Code == http.StatusCreated {
		t.Error("expected bet on locked pool to fail")
	}
}

func TestE2E_Pool_Cancel(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools", groupID), map[string]interface{}{
		"title":   "Cancel me",
		"options": []string{"A", "B"},
	}, alice)
	var pool struct {
		ID      string `json:"id"`
		Options []struct {
			ID string `json:"id"`
		} `json:"options"`
	}
	parseJSON(t, createResp, &pool)

	// Bob bets
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/bet", groupID, pool.ID), map[string]interface{}{
		"option_id": pool.Options[0].ID,
		"points":    100,
	}, bob)

	// Cancel
	cancelResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/cancel", groupID, pool.ID), nil, alice)
	if cancelResp.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d: %s", cancelResp.Code, cancelResp.Body.String())
	}

	// Verify cancelled
	getResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/pools/%s", groupID, pool.ID), nil, alice)
	var cancelled struct {
		Status string `json:"status"`
	}
	parseJSON(t, getResp, &cancelled)
	if cancelled.Status != "cancelled" {
		t.Errorf("expected status 'cancelled', got '%s'", cancelled.Status)
	}
}

func TestE2E_Pool_NonMemberCannotBet(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	charlie := env.registerUser(t, "charlie@test.com", "password123", "Charlie")

	groupID := createGroupWithMembers(t, env, alice, bob)

	createResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools", groupID), map[string]interface{}{
		"title":   "Private pool",
		"options": []string{"A", "B"},
	}, alice)
	var pool struct {
		ID      string `json:"id"`
		Options []struct {
			ID string `json:"id"`
		} `json:"options"`
	}
	parseJSON(t, createResp, &pool)

	// Charlie (not a member) tries to bet
	betResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools/%s/bet", groupID, pool.ID), map[string]interface{}{
		"option_id": pool.Options[0].ID,
		"points":    10,
	}, charlie)

	if betResp.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member bet, got %d", betResp.Code)
	}
}

func TestE2E_Pool_ListPools(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")
	groupID := createGroupWithMembers(t, env, alice, bob)

	// Create two pools
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools", groupID), map[string]interface{}{
		"title": "Pool 1", "options": []string{"A", "B"},
	}, alice)
	env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/pools", groupID), map[string]interface{}{
		"title": "Pool 2", "options": []string{"X", "Y"},
	}, alice)

	resp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s/pools", groupID), nil, alice)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var pools []struct{ ID string }
	parseJSON(t, resp, &pools)
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}
