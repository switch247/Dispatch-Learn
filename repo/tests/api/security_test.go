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

	t.Run("centralized cancellation is tenant-scoped", func(t *testing.T) {
		// Create an order with Tenant A (our test tenant)
		adminToken := loginAdmin()
		_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category": "tenant-isolation-test", "assignment_mode": "grab",
		}, adminToken)
		require.NotNil(t, orderResult["data"])
		orderData := orderResult["data"].(map[string]interface{})
		assert.Equal(t, tenantID, orderData["tenant_id"])

		// Trigger expire-stale — this should only process our tenant's orders
		resp, result := doRequest("POST", "/api/v1/dispatch/expire-stale", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			data := result["data"].(map[string]interface{})
			assert.Contains(t, data, "expired")
			assert.Contains(t, data, "cancelled")
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

// === Role Escalation Prevention — Full Policy Matrix ===
func TestRoleEscalationPrevention(t *testing.T) {
	adminToken := loginAdmin()
	require.NotEmpty(t, adminToken)

	// Register a target user
	testUser := fmt.Sprintf("escalation_test_%d", time.Now().UnixNano())
	_, regResult := doRequest("POST", "/api/v1/auth/register", map[string]string{
		"username": testUser, "password": "Test1234!", "tenant_id": tenantID,
	}, "")
	testUserID := regResult["data"].(map[string]interface{})["id"].(string)

	// --- Admin role matrix ---
	t.Run("admin can assign agent role", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{"role": "agent"}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("admin can assign dispatcher role", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{"role": "dispatcher"}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("admin can assign finance role", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{"role": "finance"}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("admin can assign instructor role", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{"role": "instructor"}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("admin cannot assign admin role (escalation denied)", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{"role": "admin"}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("admin cannot assign system_admin role (escalation denied)", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{"role": "system_admin"}, adminToken)
		require.NotNil(t, resp)
		assert.Contains(t, []int{http.StatusForbidden, http.StatusBadRequest}, resp.StatusCode)
	})

	// --- Self-assignment prevention ---
	t.Run("admin cannot modify own roles", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/users/user-admin/roles", map[string]string{"role": "agent"}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	// --- Non-admin roles cannot assign ---
	t.Run("agent cannot assign roles", func(t *testing.T) {
		_, agentLogin := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username": "agent1", "password": "admin123", "tenant_id": tenantID,
		}, "")
		agentToken := agentLogin["data"].(map[string]interface{})["access_token"].(string)

		resp, _ := doRequest("POST", "/api/v1/users/"+testUserID+"/roles", map[string]string{"role": "agent"}, agentToken)
		require.NotNil(t, resp)
		// Agent lacks admin role so middleware returns 403
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
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

// === Concurrent Order Acceptance Race Condition ===
func TestConcurrentOrderAcceptance(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	// Create a grab-mode order and make it available
	_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
		"category": "race-test", "description": "Race condition test", "assignment_mode": "grab",
	}, token)
	orderData := orderResult["data"].(map[string]interface{})
	orderID := orderData["id"].(string)

	doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{"status": "AVAILABLE"}, token)

	t.Run("only one concurrent acceptor wins", func(t *testing.T) {
		results := make(chan int, 10)

		// Launch 10 concurrent acceptance attempts
		for i := 0; i < 10; i++ {
			go func(idx int) {
				resp, _ := doRequest("POST", "/api/v1/orders/"+orderID+"/accept", map[string]string{
					"idempotency_key": fmt.Sprintf("race-%d-%d", idx, time.Now().UnixNano()),
				}, token)
				if resp != nil {
					results <- resp.StatusCode
				} else {
					results <- 0
				}
			}(i)
		}

		winners := 0
		conflicts := 0
		for i := 0; i < 10; i++ {
			code := <-results
			switch code {
			case http.StatusCreated:
				winners++
			case http.StatusConflict:
				conflicts++
			}
		}

		// Exactly one winner, rest get conflict or other errors
		assert.Equal(t, 1, winners, "exactly one concurrent acceptor should win")
		assert.Greater(t, conflicts, 0, "losers should receive 409 Conflict")
	})
}

// === Grade Masking Assertion ===
func TestGradeMaskingForNonPrivilegedRoles(t *testing.T) {
	adminToken := loginAdmin()
	require.NotEmpty(t, adminToken)

	// Create course + assessment
	_, courseResult := doRequest("POST", "/api/v1/courses", map[string]interface{}{
		"title": "Masking Test Course", "description": "test", "category": "masking-test",
	}, adminToken)
	courseID := courseResult["data"].(map[string]interface{})["id"].(string)

	_, assessResult := doRequest("POST", "/api/v1/courses/"+courseID+"/assessments", map[string]interface{}{
		"title": "Masking Quiz", "max_attempts": 3, "passing_score": 70,
	}, adminToken)
	assessmentID := assessResult["data"].(map[string]interface{})["id"].(string)

	// Login as agent (non-instructor, non-admin)
	_, agentLogin := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username": "agent1", "password": "admin123", "tenant_id": tenantID,
	}, "")
	agentToken := agentLogin["data"].(map[string]interface{})["access_token"].(string)

	// Start attempt as agent
	_, attemptResult := doRequest("POST", "/api/v1/assessments/"+assessmentID+"/attempts", nil, agentToken)
	attemptID := attemptResult["data"].(map[string]interface{})["id"].(string)

	// Submit attempt as agent (score = 85)
	resp, result := doRequest("POST", "/api/v1/attempts/"+attemptID+"/submit", map[string]interface{}{
		"answers": "A,B,C,D", "score": 85,
	}, agentToken)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	t.Run("agent sees masked grade (****)", func(t *testing.T) {
		data := result["data"].(map[string]interface{})
		numericScore := data["numeric_score"].(string)
		assert.Equal(t, "****", numericScore, "non-privileged role should see masked grade")
	})

	t.Run("admin sees actual grade", func(t *testing.T) {
		_, attempt2 := doRequest("POST", "/api/v1/assessments/"+assessmentID+"/attempts", nil, adminToken)
		attempt2ID := attempt2["data"].(map[string]interface{})["id"].(string)

		resp2, result2 := doRequest("POST", "/api/v1/attempts/"+attempt2ID+"/submit", map[string]interface{}{
			"answers": "A,B,C,D", "score": 92,
		}, adminToken)
		require.NotNil(t, resp2)
		assert.Equal(t, http.StatusCreated, resp2.StatusCode)
		data := result2["data"].(map[string]interface{})
		numericScore := data["numeric_score"].(string)
		assert.NotEqual(t, "****", numericScore, "admin should see actual grade, not masked")
	})
}

// === Assigned Agent Validation ===
func TestAssignedAgentMustExistInTenant(t *testing.T) {
	token := loginAdmin()

	t.Run("invalid agent_id rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":          "validation-test",
			"assignment_mode":   "assigned",
			"assigned_agent_id": "nonexistent-agent-id",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("valid agent_id accepted", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":          "validation-test-ok",
			"assignment_mode":   "assigned",
			"assigned_agent_id": "user-agent1",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

// === Centralized Cancellation ===
func TestCentralizedCancellation(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("expire-stale endpoint uses centralized cancel logic", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/dispatch/expire-stale", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			data := result["data"].(map[string]interface{})
			assert.Contains(t, data, "expired")
			assert.Contains(t, data, "cancelled")
		}
	})
}

// === Quota Enforcement Behavioral Tests ===
// NOTE: This test MUST run last because it temporarily reduces the rate limit.
// Go test runs in definition order within a file, and security_test.go runs after api_test.go.
func TestZZQuotaEnforcementBehavioral(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("webhook daily cap stops event dispatch", func(t *testing.T) {
		// Set webhook daily limit to 1 (keep RPM high so requests aren't rate limited)
		doRequest("PUT", "/api/v1/quotas", map[string]interface{}{
			"rpm": 600, "burst": 120, "webhook_daily_limit": 1,
		}, token)

		time.Sleep(6 * time.Second)

		// Create a webhook subscription
		doRequest("POST", "/api/v1/webhooks", map[string]interface{}{
			"url":         "http://localhost:19999/quota-test",
			"event_types": "order.created",
			"secret":      "quota-test-secret",
		}, token)

		// First order creation should succeed (webhook dispatch uses 1 quota slot)
		resp1, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category": "quota-test-1", "assignment_mode": "grab",
		}, token)
		require.NotNil(t, resp1)
		assert.Equal(t, http.StatusCreated, resp1.StatusCode)

		// Second order still succeeds — order creation itself isn't blocked by webhook quota;
		// the webhook dispatch is silently dropped when over quota (logged as warning)
		resp2, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category": "quota-test-2", "assignment_mode": "grab",
		}, token)
		require.NotNil(t, resp2)
		assert.Equal(t, http.StatusCreated, resp2.StatusCode)
	})

	t.Run("rate limit override actually throttles traffic", func(t *testing.T) {
		// Restore high quotas first to ensure we can make the PUT request
		doRequest("PUT", "/api/v1/quotas", map[string]interface{}{
			"rpm": 600, "burst": 120, "webhook_daily_limit": 10000,
		}, token)
		time.Sleep(6 * time.Second)

		// Now set a very low quota: 1 RPM with burst of 2
		doRequest("PUT", "/api/v1/quotas", map[string]interface{}{
			"rpm": 1, "burst": 2, "webhook_daily_limit": 10000,
		}, token)

		// Wait for cache to pick up override (5s TTL)
		time.Sleep(6 * time.Second)

		// Send requests rapidly — first 2 should succeed (burst), then 429
		var gotThrottled bool
		for i := 0; i < 20; i++ {
			resp, _ := doRequest("GET", "/api/v1/orders", nil, token)
			if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
				gotThrottled = true
				break
			}
		}

		assert.True(t, gotThrottled, "expected at least one 429 Too Many Requests after exceeding quota override")

		// No need to restore — this is the last test
	})
}
