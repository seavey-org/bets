package handlers

import (
	"fmt"
	"net/http"
	"testing"
)

func TestE2E_CreateGroup(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")

	resp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name":           "Test Group",
		"default_points": 500,
	}, alice)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var group struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		InviteCode string `json:"invite_code"`
	}
	parseJSON(t, resp, &group)

	if group.Name != "Test Group" {
		t.Errorf("expected name 'Test Group', got '%s'", group.Name)
	}
	if group.InviteCode == "" {
		t.Error("expected non-empty invite code")
	}
}

func TestE2E_CreateGroup_Unauthenticated(t *testing.T) {
	env := setupE2E(t)

	resp := env.doRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name":           "Test",
		"default_points": 500,
	}, nil)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.Code)
	}
}

func TestE2E_ListGroups(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")

	// Create two groups
	env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Group 1", "default_points": 100,
	}, alice)
	env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Group 2", "default_points": 200,
	}, alice)

	resp := env.authedRequest(t, "GET", "/api/groups", nil, alice)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var groups []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	parseJSON(t, resp, &groups)

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestE2E_JoinGroup(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")

	// Alice creates a group
	createResp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Join Test", "default_points": 500,
	}, alice)
	var group struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
	}
	parseJSON(t, createResp, &group)

	// Bob joins with invite code
	joinResp := env.authedRequest(t, "POST", "/api/groups/join", map[string]string{
		"invite_code": group.InviteCode,
	}, bob)

	if joinResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", joinResp.Code, joinResp.Body.String())
	}

	// Bob should be able to access the group
	getResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s", group.ID), nil, bob)
	if getResp.Code != http.StatusOK {
		t.Errorf("expected 200 accessing group as member, got %d", getResp.Code)
	}
}

func TestE2E_GroupAccess_NonMember(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")

	// Alice creates a group
	createResp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Private Group", "default_points": 500,
	}, alice)
	var group struct {
		ID string `json:"id"`
	}
	parseJSON(t, createResp, &group)

	// Bob tries to access without joining
	resp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s", group.ID), nil, bob)
	if resp.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member, got %d", resp.Code)
	}
}

func TestE2E_GrantPoints(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")

	// Alice creates group (becomes admin)
	createResp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Grant Test", "default_points": 100,
	}, alice)
	var group struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
	}
	parseJSON(t, createResp, &group)

	// Bob joins
	env.authedRequest(t, "POST", "/api/groups/join", map[string]string{
		"invite_code": group.InviteCode,
	}, bob)

	// Alice grants Bob 50 points
	grantResp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/grant", group.ID), map[string]interface{}{
		"user_id": bob.ID,
		"amount":  50,
	}, alice)

	if grantResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", grantResp.Code, grantResp.Body.String())
	}
}

func TestE2E_GrantPoints_NonAdmin(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")

	createResp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Admin Test", "default_points": 100,
	}, alice)
	var group struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
	}
	parseJSON(t, createResp, &group)

	env.authedRequest(t, "POST", "/api/groups/join", map[string]string{
		"invite_code": group.InviteCode,
	}, bob)

	// Bob (non-admin) tries to grant points
	resp := env.authedRequest(t, "POST", fmt.Sprintf("/api/groups/%s/grant", group.ID), map[string]interface{}{
		"user_id": alice.ID,
		"amount":  50,
	}, bob)

	if resp.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin grant, got %d", resp.Code)
	}
}

func TestE2E_KickMember(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")
	bob := env.registerUser(t, "bob@test.com", "password123", "Bob")

	createResp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Kick Test", "default_points": 100,
	}, alice)
	var group struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
	}
	parseJSON(t, createResp, &group)

	env.authedRequest(t, "POST", "/api/groups/join", map[string]string{
		"invite_code": group.InviteCode,
	}, bob)

	// Alice kicks Bob
	kickResp := env.authedRequest(t, "DELETE", fmt.Sprintf("/api/groups/%s/members/%s", group.ID, bob.ID), nil, alice)
	if kickResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", kickResp.Code, kickResp.Body.String())
	}

	// Bob can no longer access the group
	accessResp := env.authedRequest(t, "GET", fmt.Sprintf("/api/groups/%s", group.ID), nil, bob)
	if accessResp.Code != http.StatusForbidden {
		t.Errorf("expected 403 after kick, got %d", accessResp.Code)
	}
}

func TestE2E_DeleteGroup(t *testing.T) {
	env := setupE2E(t)
	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")

	createResp := env.authedRequest(t, "POST", "/api/groups", map[string]interface{}{
		"name": "Delete Me", "default_points": 100,
	}, alice)
	var group struct {
		ID string `json:"id"`
	}
	parseJSON(t, createResp, &group)

	// Delete
	deleteResp := env.authedRequest(t, "DELETE", fmt.Sprintf("/api/groups/%s", group.ID), nil, alice)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	// Should no longer appear in list
	listResp := env.authedRequest(t, "GET", "/api/groups", nil, alice)
	var groups []struct{ ID string }
	parseJSON(t, listResp, &groups)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups after delete, got %d", len(groups))
	}
}
