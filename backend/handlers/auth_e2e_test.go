package handlers

import (
	"net/http"
	"testing"

	"github.com/codyseavey/bets/middleware"
)

func TestE2E_Register(t *testing.T) {
	env := setupE2E(t)

	resp := env.doRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "alice@test.com",
		"password": "password123",
		"name":     "Alice",
	}, nil)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var user struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	parseJSON(t, resp, &user)

	if user.Email != "alice@test.com" {
		t.Errorf("expected email alice@test.com, got %s", user.Email)
	}
	if user.Name != "Alice" {
		t.Errorf("expected name Alice, got %s", user.Name)
	}
	if user.ID == "" {
		t.Error("expected non-empty user ID")
	}

	// Should have set an auth cookie
	cookie := extractCookie(resp, middleware.CookieName)
	if cookie == nil {
		t.Fatal("expected auth cookie to be set")
	}
}

func TestE2E_Register_DuplicateEmail(t *testing.T) {
	env := setupE2E(t)

	env.registerUser(t, "alice@test.com", "password123", "Alice")

	resp := env.doRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "alice@test.com",
		"password": "password456",
		"name":     "Alice 2",
	}, nil)

	if resp.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate email, got %d", resp.Code)
	}
}

func TestE2E_Register_BadInput(t *testing.T) {
	env := setupE2E(t)

	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing email", map[string]string{"password": "password123", "name": "X"}},
		{"missing password", map[string]string{"email": "a@b.com", "name": "X"}},
		{"missing name", map[string]string{"email": "a@b.com", "password": "password123"}},
		{"short password", map[string]string{"email": "a@b.com", "password": "short", "name": "X"}},
		{"invalid email", map[string]string{"email": "notanemail", "password": "password123", "name": "X"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := env.doRequest(t, "POST", "/api/auth/register", tt.body, nil)
			if resp.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestE2E_Login(t *testing.T) {
	env := setupE2E(t)

	env.registerUser(t, "alice@test.com", "password123", "Alice")

	resp := env.doRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "alice@test.com",
		"password": "password123",
	}, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	cookie := extractCookie(resp, middleware.CookieName)
	if cookie == nil {
		t.Fatal("expected auth cookie on login")
	}
}

func TestE2E_Login_WrongPassword(t *testing.T) {
	env := setupE2E(t)

	env.registerUser(t, "alice@test.com", "password123", "Alice")

	resp := env.doRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "alice@test.com",
		"password": "wrongpassword",
	}, nil)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.Code)
	}
}

func TestE2E_Login_NonexistentUser(t *testing.T) {
	env := setupE2E(t)

	resp := env.doRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "nobody@test.com",
		"password": "password123",
	}, nil)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.Code)
	}
}

func TestE2E_Me(t *testing.T) {
	env := setupE2E(t)

	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")

	resp := env.authedRequest(t, "GET", "/api/auth/me", nil, alice)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var user struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	parseJSON(t, resp, &user)

	if user.ID != alice.ID {
		t.Errorf("expected user ID %s, got %s", alice.ID, user.ID)
	}
	if user.Email != "alice@test.com" {
		t.Errorf("expected email alice@test.com, got %s", user.Email)
	}
}

func TestE2E_Me_Unauthenticated(t *testing.T) {
	env := setupE2E(t)

	resp := env.doRequest(t, "GET", "/api/auth/me", nil, nil)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.Code)
	}
}

func TestE2E_Logout(t *testing.T) {
	env := setupE2E(t)

	alice := env.registerUser(t, "alice@test.com", "password123", "Alice")

	resp := env.authedRequest(t, "POST", "/api/auth/logout", nil, alice)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Cookie should be cleared (MaxAge < 0)
	cookie := extractCookie(resp, middleware.CookieName)
	if cookie != nil && cookie.MaxAge > 0 {
		t.Error("expected auth cookie to be cleared on logout")
	}
}
