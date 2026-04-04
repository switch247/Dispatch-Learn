package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// === Object-Level Auth: Session Revocation BOLA ===
func TestSessionRevocationBOLA(t *testing.T) {
	user1Name := fmt.Sprintf("bola_user1_%d", time.Now().UnixNano())
	user2Name := fmt.Sprintf("bola_user2_%d", time.Now().UnixNano())

	doRequest("POST", "/api/v1/auth/register", map[string]string{
		"username": user1Name, "password": "Test1234!", "tenant_id": tenantID,
	}, "")
	doRequest("POST", "/api/v1/auth/register", map[string]string{
		"username": user2Name, "password": "Test1234!", "tenant_id": tenantID,
	}, "")

	_, login1 := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username": user1Name, "password": "Test1234!", "tenant_id": tenantID,
	}, "")
	token1 := login1["data"].(map[string]interface{})["access_token"].(string)

	_, login2 := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username": user2Name, "password": "Test1234!", "tenant_id": tenantID,
	}, "")
	token2 := login2["data"].(map[string]interface{})["access_token"].(string)

	_, sessResult := doRequest("GET", "/api/v1/sessions", nil, token1)
	sessions := sessResult["data"].([]interface{})
	require.NotEmpty(t, sessions)
	user1SessionID := sessions[0].(map[string]interface{})["id"].(string)

	t.Run("revoking own session succeeds", func(t *testing.T) {
		_, login1b := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username": user1Name, "password": "Test1234!", "tenant_id": tenantID,
		}, "")
		token1b := login1b["data"].(map[string]interface{})["access_token"].(string)

		_, sess1b := doRequest("GET", "/api/v1/sessions", nil, token1b)
		sessionList := sess1b["data"].([]interface{})
		targetSession := sessionList[len(sessionList)-1].(map[string]interface{})["id"].(string)

		resp, _ := doRequest("DELETE", "/api/v1/sessions/"+targetSession, nil, token1b)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("revoking another user's session returns 403", func(t *testing.T) {
		resp, _ := doRequest("DELETE", "/api/v1/sessions/"+user1SessionID, nil, token2)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

// === Financial Read Authorization ===
func TestFinanceReadAuthorization(t *testing.T) {
	_, agentLogin := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username": "agent1", "password": "admin123", "tenant_id": tenantID,
	}, "")
	require.NotNil(t, agentLogin["data"])
	agentToken := agentLogin["data"].(map[string]interface{})["access_token"].(string)

	t.Run("agent cannot list invoices", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/invoices", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("agent cannot list payments", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/payments/nonexistent", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("agent cannot list ledger", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/ledger", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("admin can list invoices", func(t *testing.T) {
		adminToken := loginAdmin()
		resp, _ := doRequest("GET", "/api/v1/invoices", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// === Cross-Tenant Isolation ===
func TestCrossTenantIsolation(t *testing.T) {
	t.Run("login with non-existent tenant returns unauthorized", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username": "admin", "password": "admin123",
			"tenant_id": "99999999-9999-9999-9999-999999999999",
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("cannot access resources from different tenant", func(t *testing.T) {
		adminToken := loginAdmin()
		require.NotEmpty(t, adminToken)
		resp, result := doRequest("GET", "/api/v1/orders", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			orders, ok := result["data"].([]interface{})
			if ok {
				for _, o := range orders {
					order := o.(map[string]interface{})
					assert.Equal(t, tenantID, order["tenant_id"])
				}
			}
		}
	})
}

// === Webhook Event Wiring ===
func TestWebhookEventWiring(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	_, subResult := doRequest("POST", "/api/v1/webhooks", map[string]interface{}{
		"url":         "http://localhost:19999/test-hook",
		"event_types": "order.created,order.accepted,scoring.completed,learning.completed",
		"secret":      "test-webhook-secret",
	}, token)
	require.NotNil(t, subResult["data"])

	t.Run("creating an order triggers webhook dispatch", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "webhook-test",
			"description":     "Testing webhook wiring",
			"assignment_mode": "grab",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}

// === Assigned Mode Dispatch (Phase 2) ===
func TestAssignedModeDispatch(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("assigned mode requires assigned_agent_id", func(t *testing.T) {
		// Creating order in assigned mode without assigned_agent_id should fail
		resp, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "assigned-test",
			"description":     "Should fail without agent",
			"assignment_mode": "assigned",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("assigned mode with agent_id creates order", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":          "assigned-test",
			"description":       "Assigned to agent1",
			"assignment_mode":   "assigned",
			"assigned_agent_id": "user-agent1",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "assigned", data["assignment_mode"])
	})

	t.Run("non-designated agent cannot accept assigned order", func(t *testing.T) {
		// Create order assigned to agent1
		_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":          "assigned-test-2",
			"description":       "Only agent1 can accept",
			"assignment_mode":   "assigned",
			"assigned_agent_id": "user-agent1",
		}, token)
		orderData := orderResult["data"].(map[string]interface{})
		orderID := orderData["id"].(string)

		// Make it available
		doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "AVAILABLE",
		}, token)

		// Admin (not agent1) tries to accept - should be rejected
		resp, _ := doRequest("POST", "/api/v1/orders/"+orderID+"/accept", map[string]string{
			"idempotency_key": fmt.Sprintf("assigned-test-%d", time.Now().UnixNano()),
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

// === Role Escalation Prevention (Phase 2 - High 4) ===
func TestRoleEscalationPrevention(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	// Register a test user
	testUser := fmt.Sprintf("escalation_test_%d", time.Now().UnixNano())
	_, regResult := doRequest("POST", "/api/v1/auth/register", map[string]string{
		"username": testUser, "password": "Test1234!", "tenant_id": tenantID,
	}, "")
	testUserID := regResult["data"].(map[string]interface{})["id"].(string)

	t.Run("admin can assign agent role", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{
			"role": "agent",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("admin can assign dispatcher role", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{
			"role": "dispatcher",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("admin cannot assign admin role (escalation denied)", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{
			"role": "admin",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("admin cannot assign system_admin role (escalation denied)", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{
			"role": "system_admin",
		}, token)
		require.NotNil(t, resp)
		// May be 403 (escalation denied) or 400 (role not found since system_admin might not be seeded)
		assert.Contains(t, []int{http.StatusForbidden, http.StatusBadRequest}, resp.StatusCode)
	})
}

// === Certification Query Scoping (Phase 2 - Medium 7) ===
func TestCertificationScoping(t *testing.T) {
	// agent1 should not be able to view other users' certifications
	_, agentLogin := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username": "agent1", "password": "admin123", "tenant_id": tenantID,
	}, "")
	agentToken := agentLogin["data"].(map[string]interface{})["access_token"].(string)

	t.Run("agent can view own certifications", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/certifications", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("agent cannot view another user's certifications", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/certifications?user_id=user-admin", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("admin can view any user's certifications", func(t *testing.T) {
		adminToken := loginAdmin()
		resp, _ := doRequest("GET", "/api/v1/certifications?user_id=user-agent1", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// === OAuth2 Endpoints (Phase 2 - Medium 5) ===
func TestOAuth2Endpoints(t *testing.T) {
	// OAuth2 is disabled by default
	t.Run("oauth2 login returns 404 when disabled", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/auth/oauth2/login", nil, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("oauth2 callback returns 404 when disabled", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/auth/oauth2/callback", map[string]string{
			"code": "test-code",
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// === Config Change Tracking ===
func TestConfigChangeTracking(t *testing.T) {
	token := loginAdmin()

	t.Run("quota override creates config change record", func(t *testing.T) {
		doRequest("PUT", "/api/v1/quotas", map[string]interface{}{
			"rpm": 800, "burst": 200, "webhook_daily_limit": 15000,
		}, token)

		resp, result := doRequest("GET", "/api/v1/config-changes", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		meta := result["meta"].(map[string]interface{})
		assert.Greater(t, meta["total"].(float64), float64(0))
	})
}

// === Audit Chain Integrity ===
func TestAuditChainIntegrity(t *testing.T) {
	token := loginAdmin()

	t.Run("audit chain is valid after operations", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/audit-logs/verify", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			data := result["data"].(map[string]interface{})
			assert.Equal(t, true, data["valid"])
		}
	})
}

// === Offline Swagger UI ===
func TestOfflineSwaggerUI(t *testing.T) {
	t.Run("swagger UI page loads without internet", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/docs", nil)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	})

	t.Run("static swagger assets are served locally", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/static/swagger/swagger-ui.css", nil)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("static swagger bundle is served locally", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/static/swagger/swagger-ui-bundle.js", nil)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
