//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type loginData struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	User         userData `json:"user"`
}

type userData struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
}

func TestRegisterAndLogin(t *testing.T) {
	ts := NewTestServer(t)

	// Step 1: Register a new account
	regBody := map[string]string{
		"email":        "test@example.com",
		"password":     "password123",
		"display_name": "Test User",
	}

	resp := ts.PostJSON(t, "/api/v1/auth/register", regBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected status 201, got %d", resp.StatusCode)
	}

	var regResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		t.Fatalf("register: failed to decode response: %v", err)
	}

	if !regResp.Success {
		t.Fatalf("register: expected success=true, got false (error: %s)", regResp.Error)
	}

	var regUser userData
	if err := json.Unmarshal(regResp.Data, &regUser); err != nil {
		t.Fatalf("register: failed to decode user data: %v", err)
	}

	if regUser.Email != "test@example.com" {
		t.Errorf("register: expected email test@example.com, got %s", regUser.Email)
	}
	if regUser.DisplayName != "Test User" {
		t.Errorf("register: expected display_name Test User, got %s", regUser.DisplayName)
	}
	if regUser.ID == "" {
		t.Error("register: expected non-empty user ID")
	}

	// Step 2: Login with the new account
	loginBody := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}

	resp2 := ts.PostJSON(t, "/api/v1/auth/login", loginBody)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("login: expected status 200, got %d", resp2.StatusCode)
	}

	var loginResp apiResponse
	if err := json.NewDecoder(resp2.Body).Decode(&loginResp); err != nil {
		t.Fatalf("login: failed to decode response: %v", err)
	}

	if !loginResp.Success {
		t.Fatalf("login: expected success=true, got false (error: %s)", loginResp.Error)
	}

	var login loginData
	if err := json.Unmarshal(loginResp.Data, &login); err != nil {
		t.Fatalf("login: failed to decode login data: %v", err)
	}

	if login.AccessToken == "" {
		t.Error("login: expected non-empty access_token")
	}
	if login.RefreshToken == "" {
		t.Error("login: expected non-empty refresh_token")
	}
	if login.User.Email != "test@example.com" {
		t.Errorf("login: expected user email test@example.com, got %s", login.User.Email)
	}

	// Step 3: Access protected endpoint with the token
	resp3 := ts.GetWithToken(t, "/api/v1/me/", login.AccessToken)
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("me: expected status 200, got %d", resp3.StatusCode)
	}

	var meResp apiResponse
	if err := json.NewDecoder(resp3.Body).Decode(&meResp); err != nil {
		t.Fatalf("me: failed to decode response: %v", err)
	}

	if !meResp.Success {
		t.Fatalf("me: expected success=true, got false (error: %s)", meResp.Error)
	}

	var meUser userData
	if err := json.Unmarshal(meResp.Data, &meUser); err != nil {
		t.Fatalf("me: failed to decode user data: %v", err)
	}

	if meUser.Email != "test@example.com" {
		t.Errorf("me: expected email test@example.com, got %s", meUser.Email)
	}
	if meUser.DisplayName != "Test User" {
		t.Errorf("me: expected display_name Test User, got %s", meUser.DisplayName)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	ts := NewTestServer(t)

	// Register a user first
	regBody := map[string]string{
		"email":        "user@example.com",
		"password":     "correctpassword",
		"display_name": "User",
	}
	resp := ts.PostJSON(t, "/api/v1/auth/register", regBody)
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected status 201, got %d", resp.StatusCode)
	}

	// Attempt login with wrong password
	loginBody := map[string]string{
		"email":    "user@example.com",
		"password": "wrongpassword",
	}
	resp2 := ts.PostJSON(t, "/api/v1/auth/login", loginBody)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login: expected status 401, got %d", resp2.StatusCode)
	}

	var loginResp apiResponse
	if err := json.NewDecoder(resp2.Body).Decode(&loginResp); err != nil {
		t.Fatalf("login: failed to decode response: %v", err)
	}

	if loginResp.Success {
		t.Error("login: expected success=false for invalid credentials")
	}
	if loginResp.Error != "invalid credentials" {
		t.Errorf("login: expected error 'invalid credentials', got %q", loginResp.Error)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	ts := NewTestServer(t)

	body := map[string]string{
		"email":        "dup@example.com",
		"password":     "password123",
		"display_name": "First User",
	}

	// First registration should succeed
	resp := ts.PostJSON(t, "/api/v1/auth/register", body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first register: expected status 201, got %d", resp.StatusCode)
	}

	// Second registration with the same email should fail
	resp2 := ts.PostJSON(t, "/api/v1/auth/register", map[string]string{
		"email":        "dup@example.com",
		"password":     "password123",
		"display_name": "Second User",
	})
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate register: expected status 409, got %d", resp2.StatusCode)
	}

	var regResp apiResponse
	if err := json.NewDecoder(resp2.Body).Decode(&regResp); err != nil {
		t.Fatalf("duplicate register: failed to decode response: %v", err)
	}

	if regResp.Success {
		t.Error("duplicate register: expected success=false")
	}
	if regResp.Error != "email already exists" {
		t.Errorf("duplicate register: expected error 'email already exists', got %q", regResp.Error)
	}
}

func TestProtectedEndpointWithoutToken(t *testing.T) {
	ts := NewTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/me/")
	if err != nil {
		t.Fatalf("GET /api/v1/me/ failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me without token: expected status 401, got %d", resp.StatusCode)
	}
}

// registerAndLogin is a helper that registers a user and logs in, returning the tokens.
func registerAndLogin(t *testing.T, ts *TestServer, email, password, displayName string) loginData {
	t.Helper()

	resp := ts.PostJSON(t, "/api/v1/auth/register", map[string]string{
		"email":        email,
		"password":     password,
		"display_name": displayName,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected status 201, got %d", resp.StatusCode)
	}

	resp2 := ts.PostJSON(t, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("login: expected status 200, got %d", resp2.StatusCode)
	}

	var loginResp apiResponse
	if err := json.NewDecoder(resp2.Body).Decode(&loginResp); err != nil {
		t.Fatalf("login: failed to decode response: %v", err)
	}

	var data loginData
	if err := json.Unmarshal(loginResp.Data, &data); err != nil {
		t.Fatalf("login: failed to decode login data: %v", err)
	}

	return data
}

func TestTokenRefresh(t *testing.T) {
	ts := NewTestServer(t)
	login := registerAndLogin(t, ts, "refresh@example.com", "password123", "Refresh User")

	// Use the refresh token to get new tokens
	resp := ts.PostJSON(t, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": login.RefreshToken,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: expected status 200, got %d", resp.StatusCode)
	}

	var refreshResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&refreshResp); err != nil {
		t.Fatalf("refresh: failed to decode response: %v", err)
	}

	if !refreshResp.Success {
		t.Fatalf("refresh: expected success=true, got false (error: %s)", refreshResp.Error)
	}

	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(refreshResp.Data, &tokens); err != nil {
		t.Fatalf("refresh: failed to decode token data: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("refresh: expected non-empty access_token")
	}
	if tokens.RefreshToken == "" {
		t.Error("refresh: expected non-empty refresh_token")
	}

	// Verify the new access token works
	resp2 := ts.GetWithToken(t, "/api/v1/me/", tokens.AccessToken)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("me with refreshed token: expected status 200, got %d", resp2.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	ts := NewTestServer(t)

	// Logout is stateless (JWT), so it always succeeds
	resp := ts.PostJSON(t, "/api/v1/auth/logout", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout: expected status 200, got %d", resp.StatusCode)
	}

	var logoutResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&logoutResp); err != nil {
		t.Fatalf("logout: failed to decode response: %v", err)
	}

	if !logoutResp.Success {
		t.Fatalf("logout: expected success=true, got false (error: %s)", logoutResp.Error)
	}
}

func TestPasswordChange(t *testing.T) {
	ts := NewTestServer(t)
	login := registerAndLogin(t, ts, "pwchange@example.com", "oldpassword1", "PW User")

	// Change the password
	resp := ts.PutJSONWithToken(t, "/api/v1/me/password", map[string]string{
		"current_password": "oldpassword1",
		"new_password":     "newpassword1",
	}, login.AccessToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("password change: expected status 200, got %d", resp.StatusCode)
	}

	// Login with old password should fail
	resp2 := ts.PostJSON(t, "/api/v1/auth/login", map[string]string{
		"email":    "pwchange@example.com",
		"password": "oldpassword1",
	})
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login with old password: expected status 401, got %d", resp2.StatusCode)
	}

	// Login with new password should succeed
	resp3 := ts.PostJSON(t, "/api/v1/auth/login", map[string]string{
		"email":    "pwchange@example.com",
		"password": "newpassword1",
	})
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("login with new password: expected status 200, got %d", resp3.StatusCode)
	}
}
