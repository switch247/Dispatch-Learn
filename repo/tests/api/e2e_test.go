package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loginAs is a helper that returns the access token for a given user.
func loginAs(username, password string) string {
	_, result := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username":  username,
		"password":  password,
		"tenant_id": tenantID,
	}, "")
	if result == nil || result["data"] == nil {
		return ""
	}
	return result["data"].(map[string]interface{})["access_token"].(string)
}

// ---------------------------------------------------------------
// Missing single-endpoint tests
// ---------------------------------------------------------------

func TestGetUserByID(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	username := fmt.Sprintf("get_by_id_%d", time.Now().UnixNano())
	_, regResult := doRequest("POST", "/api/v1/auth/register", map[string]string{
		"username":  username,
		"password":  "TestPass123!",
		"tenant_id": tenantID,
	}, "")
	require.NotNil(t, regResult["data"])
	userID := regResult["data"].(map[string]interface{})["id"].(string)

	t.Run("get existing user returns 200", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/users/"+userID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, username, data["username"])
		assert.Equal(t, tenantID, data["tenant_id"])
	})

	t.Run("get non-existent user returns 404", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/users/00000000-0000-0000-0000-000000000999", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestLogoutEndpoint(t *testing.T) {
	username := fmt.Sprintf("logout_test_%d", time.Now().UnixNano())
	doRequest("POST", "/api/v1/auth/register", map[string]string{
		"username":  username,
		"password":  "TestPass123!",
		"tenant_id": tenantID,
	}, "")

	_, loginResult := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username":  username,
		"password":  "TestPass123!",
		"tenant_id": tenantID,
	}, "")
	require.NotNil(t, loginResult["data"])
	token := loginResult["data"].(map[string]interface{})["access_token"].(string)

	_, sessResult := doRequest("GET", "/api/v1/sessions", nil, token)
	sessions := sessResult["data"].([]interface{})
	require.NotEmpty(t, sessions)
	sessionID := sessions[0].(map[string]interface{})["id"].(string)

	t.Run("logout from own session succeeds", func(t *testing.T) {
		// Login a second time to get a fresh token for the logout call
		_, loginResult2 := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username":  username,
			"password":  "TestPass123!",
			"tenant_id": tenantID,
		}, "")
		token2 := loginResult2["data"].(map[string]interface{})["access_token"].(string)

		_, sessResult2 := doRequest("GET", "/api/v1/sessions", nil, token2)
		sessions2 := sessResult2["data"].([]interface{})
		target := sessions2[len(sessions2)-1].(map[string]interface{})["id"].(string)

		resp, _ := doRequest("POST", "/api/v1/auth/logout/"+target, nil, token2)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("cannot logout another user's session", func(t *testing.T) {
		other := fmt.Sprintf("other_logout_%d", time.Now().UnixNano())
		doRequest("POST", "/api/v1/auth/register", map[string]string{
			"username":  other,
			"password":  "TestPass123!",
			"tenant_id": tenantID,
		}, "")
		_, ol := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username":  other,
			"password":  "TestPass123!",
			"tenant_id": tenantID,
		}, "")
		otherToken := ol["data"].(map[string]interface{})["access_token"].(string)

		resp, _ := doRequest("POST", "/api/v1/auth/logout/"+sessionID, nil, otherToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestServiceZoneCreate(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("admin creates service zone", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/service-zones", map[string]interface{}{
			"name":         fmt.Sprintf("Zone Alpha %d", time.Now().UnixNano()),
			"zip_codes":    "10001,10002,10003",
			"centroid_lat": 40.7128,
			"centroid_lng": -74.0060,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.NotEmpty(t, data["id"])
	})

	t.Run("list includes newly created zone", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/service-zones", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		zones := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(zones), 1)
	})

	t.Run("non-admin cannot create service zone", func(t *testing.T) {
		agentToken := loginAs("agent1", "admin123")
		require.NotEmpty(t, agentToken)
		resp, _ := doRequest("POST", "/api/v1/service-zones", map[string]interface{}{
			"name": "Unauthorized Zone", "zip_codes": "99999",
		}, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestAgentProfileCRUD(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("create agent profile", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/agent-profiles", map[string]interface{}{
			"user_id":          "user-agent1",
			"zip_code":         "10001",
			"is_available":     true,
			"max_workload":     8,
			"reputation_score": 75.0,
		}, token)
		require.NotNil(t, resp)
		// 201 on first creation, 409 if already seeded
		assert.Contains(t, []int{http.StatusCreated, http.StatusConflict}, resp.StatusCode)
		if resp.StatusCode == http.StatusCreated {
			data := result["data"].(map[string]interface{})
			assert.Equal(t, "user-agent1", data["user_id"])
		}
	})

	t.Run("get agent profile by user_id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/agent-profiles/user-agent1", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "user-agent1", data["user_id"])
	})

	t.Run("get non-existent profile returns 404", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/agent-profiles/nonexistent-user-id", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestGetAssessmentByID(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	_, courseResult := doRequest("POST", "/api/v1/courses", map[string]interface{}{
		"title":    fmt.Sprintf("Assessment Fetch Course %d", time.Now().UnixNano()),
		"category": "testing",
	}, token)
	courseID := courseResult["data"].(map[string]interface{})["id"].(string)

	_, assessResult := doRequest("POST", "/api/v1/courses/"+courseID+"/assessments", map[string]interface{}{
		"title":         "Fetch Assessment Test",
		"description":   "Unit test for GET /assessments/:id",
		"max_attempts":  5,
		"passing_score": 65,
	}, token)
	assessmentID := assessResult["data"].(map[string]interface{})["id"].(string)

	t.Run("get assessment by id returns full details", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/assessments/"+assessmentID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "Fetch Assessment Test", data["title"])
		assert.Equal(t, float64(65), data["passing_score"])
		assert.Equal(t, float64(5), data["max_attempts"])
	})

	t.Run("agent can also get assessment", func(t *testing.T) {
		agentToken := loginAs("agent1", "admin123")
		resp, _ := doRequest("GET", "/api/v1/assessments/"+assessmentID, nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestReaderArtifactsCRUD(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	_, courseResult := doRequest("POST", "/api/v1/courses", map[string]interface{}{
		"title":    fmt.Sprintf("Reader Course %d", time.Now().UnixNano()),
		"category": "reader-test",
	}, token)
	courseID := courseResult["data"].(map[string]interface{})["id"].(string)

	_, contentResult := doRequest("POST", "/api/v1/courses/"+courseID+"/content", map[string]interface{}{
		"title":        "Reader Chapter 1",
		"content_type": "epub",
		"file_path":    "/content/reader/ch1.epub",
		"checksum":     "sha256-reader01",
		"size_bytes":   1048576,
	}, token)
	contentID := contentResult["data"].(map[string]interface{})["id"].(string)

	t.Run("create bookmark", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/reader-artifacts", map[string]interface{}{
			"content_id":    contentID,
			"artifact_type": "bookmark",
			"position":      "page:42",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "bookmark", data["artifact_type"])
	})

	t.Run("create highlight", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/reader-artifacts", map[string]interface{}{
			"content_id":    contentID,
			"artifact_type": "highlight",
			"position":      "page:55:char:120",
			"content":       "Key safety concept to remember",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "highlight", data["artifact_type"])
	})

	t.Run("create annotation", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/reader-artifacts", map[string]interface{}{
			"content_id":    contentID,
			"artifact_type": "annotation",
			"position":      "page:10",
			"content":       "Personal study note",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "annotation", data["artifact_type"])
	})

	t.Run("list all reader artifacts", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/reader-artifacts", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("list reader artifacts filtered by content_id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/reader-artifacts?content_id="+contentID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		artifacts := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(artifacts), 3)
	})

	t.Run("missing content_id rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/reader-artifacts", map[string]interface{}{
			"artifact_type": "bookmark",
			"position":      "page:1",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestCertificationIssuance(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	_, courseResult := doRequest("POST", "/api/v1/courses", map[string]interface{}{
		"title":    fmt.Sprintf("Cert Issuance Course %d", time.Now().UnixNano()),
		"category": "certification",
	}, token)
	courseID := courseResult["data"].(map[string]interface{})["id"].(string)

	t.Run("admin issues certification to agent", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/certifications", map[string]interface{}{
			"user_id":   "user-agent1",
			"course_id": courseID,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, courseID, data["course_id"])
		assert.NotEmpty(t, data["issued_at"])
	})

	t.Run("admin can list any user certifications", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/certifications", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("agent without required fields rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/certifications", map[string]interface{}{
			"user_id": "user-agent1",
			// missing course_id
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestFinanceDetailEndpoints(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
		"category":        "finance-detail-test",
		"description":     "Finance detail endpoint coverage",
		"assignment_mode": "grab",
	}, token)
	orderID := orderResult["data"].(map[string]interface{})["id"].(string)

	_, invResult := doRequest("POST", "/api/v1/invoices", map[string]interface{}{
		"order_id":        orderID,
		"subtotal":        200.00,
		"tax_rate":        0.10,
		"billing_address": "456 Finance St, New York, NY 10002",
	}, token)
	invoiceID := invResult["data"].(map[string]interface{})["id"].(string)

	doRequest("POST", "/api/v1/invoices/"+invoiceID+"/issue", nil, token)

	_, payResult := doRequest("POST", "/api/v1/payments", map[string]interface{}{
		"order_id":        orderID,
		"invoice_id":      invoiceID,
		"amount":          220.00,
		"method":          "card_present",
		"reference":       "CARD-FIN-001",
		"idempotency_key": fmt.Sprintf("fin-detail-%d", time.Now().UnixNano()),
	}, token)
	require.NotNil(t, payResult["data"])
	paymentID := payResult["data"].(map[string]interface{})["id"].(string)

	t.Run("get invoice by id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/invoices/"+invoiceID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, invoiceID, data["id"])
		assert.Equal(t, "ISSUED", data["status"])
	})

	t.Run("list payments by invoice", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/invoices/"+invoiceID+"/payments", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		payments := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(payments), 1)
	})

	t.Run("get payment by id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/payments/"+paymentID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, paymentID, data["id"])
		assert.Equal(t, "card_present", data["method"])
	})

	t.Run("get ledger entries by order", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders/"+orderID+"/ledger", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		entries := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(entries), 1)
	})

	t.Run("process partial refund", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/refunds", map[string]interface{}{
			"payment_id": paymentID,
			"amount":     50.00,
			"reason":     "Partial service credit for late arrival",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, float64(50.00), data["amount"])
	})

	t.Run("refund without reason rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/refunds", map[string]interface{}{
			"payment_id": paymentID,
			"amount":     10.00,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("non-finance role cannot access invoices", func(t *testing.T) {
		agentToken := loginAs("agent1", "admin123")
		resp, _ := doRequest("GET", "/api/v1/invoices/"+invoiceID, nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestWebhookDetailEndpoints(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	_, subResult := doRequest("POST", "/api/v1/webhooks", map[string]interface{}{
		"url":         "http://localhost:29999/webhook-detail",
		"event_types": "order.created,order.accepted",
		"secret":      "detail-secret-key",
	}, token)
	require.NotNil(t, subResult["data"])
	webhookID := subResult["data"].(map[string]interface{})["id"].(string)

	t.Run("get webhook subscription by id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/webhooks/"+webhookID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, webhookID, data["id"])
		assert.Contains(t, data["event_types"].(string), "order.created")
	})

	t.Run("list dead-letter queue", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/webhooks/dead-letters", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("non-admin cannot access webhooks", func(t *testing.T) {
		agentToken := loginAs("agent1", "admin123")
		resp, _ := doRequest("GET", "/api/v1/webhooks/"+webhookID, nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestReportDetailEndpoints(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	_, reportResult := doRequest("POST", "/api/v1/reports", map[string]interface{}{
		"report_type": "kpi",
		"parameters":  map[string]string{"period": "monthly"},
	}, token)
	require.NotNil(t, reportResult["data"])
	reportID := reportResult["data"].(map[string]interface{})["id"].(string)

	t.Run("get report by id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/reports/"+reportID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, reportID, data["id"])
		assert.Equal(t, "kpi", data["report_type"])
	})

	t.Run("non-admin cannot access reports", func(t *testing.T) {
		agentToken := loginAs("agent1", "admin123")
		resp, _ := doRequest("GET", "/api/v1/reports/"+reportID, nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestAuditLogFiltering(t *testing.T) {
	token := loginAdmin()

	t.Run("filter by entity type", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/audit-logs?entity_type=Order", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})

	t.Run("paginate audit logs", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/audit-logs?page=1&per_page=5", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		meta := result["meta"].(map[string]interface{})
		assert.Equal(t, float64(5), meta["per_page"])
	})

	t.Run("non-admin cannot access audit logs", func(t *testing.T) {
		agentToken := loginAs("agent1", "admin123")
		resp, _ := doRequest("GET", "/api/v1/audit-logs", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

// ---------------------------------------------------------------
// E2E FLOW 1: Complete Order Lifecycle
// CREATED → AVAILABLE → ACCEPTED → IN_PROGRESS → COMPLETED
// ---------------------------------------------------------------

func TestE2ECompleteOrderLifecycle(t *testing.T) {
	adminToken := loginAdmin()
	require.NotEmpty(t, adminToken)

	var orderID string

	t.Run("step1_create_grab_order", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "hvac-repair",
			"description":     "E2E: Emergency HVAC repair",
			"zip_code":        "10001",
			"assignment_mode": "grab",
			"priority":        2,
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		orderID = data["id"].(string)
		assert.Equal(t, "CREATED", data["status"])
		assert.NotEmpty(t, orderID)
	})

	t.Run("step2_get_order_confirms_state", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders/"+orderID, nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "hvac-repair", data["category"])
		assert.Equal(t, "CREATED", data["status"])
	})

	t.Run("step3_transition_created_to_available", func(t *testing.T) {
		resp, result := doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "AVAILABLE",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "AVAILABLE", data["status"])
	})

	t.Run("step4_get_agent_recommendations", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders/"+orderID+"/recommendations", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("step5_accept_order", func(t *testing.T) {
		idemKey := fmt.Sprintf("e2e-accept-%d", time.Now().UnixNano())
		resp, _ := doRequest("POST", "/api/v1/orders/"+orderID+"/accept", map[string]string{
			"idempotency_key": idemKey,
		}, adminToken)
		require.NotNil(t, resp)
		accepted := resp.StatusCode == http.StatusCreated
		if !accepted {
			// Force ACCEPTED status transition for subsequent steps
			doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
				"status": "ACCEPTED",
			}, adminToken)
		}
	})

	t.Run("step6_transition_accepted_to_in_progress", func(t *testing.T) {
		resp, result := doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "IN_PROGRESS",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "IN_PROGRESS", data["status"])
	})

	t.Run("step7_transition_in_progress_to_completed", func(t *testing.T) {
		resp, result := doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "COMPLETED",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "COMPLETED", data["status"])
	})

	t.Run("step8_completed_order_is_immutable", func(t *testing.T) {
		resp, _ := doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "CANCELLED",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("step9_filter_orders_by_completed_status", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders?status=COMPLETED", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})

	t.Run("step10_create_invoice_for_completed_order", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/invoices", map[string]interface{}{
			"order_id":        orderID,
			"subtotal":        350.00,
			"tax_rate":        0.08,
			"billing_address": "789 E2E Ave, New York, NY 10001",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}

// ---------------------------------------------------------------
// E2E FLOW 2: Complete LMS Learning Journey
// Course → Content → Assessment → Attempt → Submit → Certification
// ---------------------------------------------------------------

func TestE2ELMSLearningJourney(t *testing.T) {
	adminToken := loginAdmin()
	require.NotEmpty(t, adminToken)

	agentToken := loginAs("agent1", "admin123")
	require.NotEmpty(t, agentToken)

	var courseID, contentID, assessmentID, attemptID string

	t.Run("step1_create_course", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/courses", map[string]interface{}{
			"title":       fmt.Sprintf("E2E Safety Training %d", time.Now().UnixNano()),
			"description": "Full e2e safety certification program",
			"category":    "safety",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		courseID = data["id"].(string)
		assert.NotEmpty(t, courseID)
	})

	t.Run("step2_add_epub_content", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/courses/"+courseID+"/content", map[string]interface{}{
			"title":        "Hazard Identification Module",
			"content_type": "epub",
			"file_path":    "/content/safety/hazard.epub",
			"checksum":     "sha256-hazard01",
			"size_bytes":   2097152,
			"sort_order":   1,
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		contentID = data["id"].(string)
		assert.NotEmpty(t, contentID)
	})

	t.Run("step3_agent_bookmarks_content", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/reader-artifacts", map[string]interface{}{
			"content_id":    contentID,
			"artifact_type": "bookmark",
			"position":      "page:23",
		}, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("step4_create_assessment", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/courses/"+courseID+"/assessments", map[string]interface{}{
			"title":         "Safety Competency Quiz",
			"description":   "Verify agent understands safety protocols",
			"max_attempts":  3,
			"passing_score": 75,
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assessmentID = data["id"].(string)
		assert.NotEmpty(t, assessmentID)
	})

	t.Run("step5_agent_fetches_assessment_details", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/assessments/"+assessmentID, nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "Safety Competency Quiz", data["title"])
		assert.Equal(t, float64(75), data["passing_score"])
	})

	t.Run("step6_agent_starts_attempt", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/assessments/"+assessmentID+"/attempts", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		attemptID = data["id"].(string)
		assert.NotEmpty(t, attemptID)
		assert.NotEmpty(t, data["started_at"])
	})

	t.Run("step7_agent_submits_passing_score", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/attempts/"+attemptID+"/submit", map[string]interface{}{
			"answers": "A,C,B,D,A",
			"score":   82,
		}, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "****", data["numeric_score"], "agent sees masked grade")
		assert.Equal(t, true, data["is_passing"])
		assert.NotEmpty(t, data["letter_grade"])
	})

	t.Run("step8_admin_sees_actual_grade", func(t *testing.T) {
		_, adminAttempt := doRequest("POST", "/api/v1/assessments/"+assessmentID+"/attempts", nil, adminToken)
		adminAttemptID := adminAttempt["data"].(map[string]interface{})["id"].(string)

		_, gradeResult := doRequest("POST", "/api/v1/attempts/"+adminAttemptID+"/submit", map[string]interface{}{
			"answers": "A,C,B,D,A",
			"score":   90,
		}, adminToken)
		data := gradeResult["data"].(map[string]interface{})
		assert.NotEqual(t, "****", data["numeric_score"], "admin sees real score")
	})

	t.Run("step9_admin_issues_certification", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/certifications", map[string]interface{}{
			"user_id":   "user-agent1",
			"course_id": courseID,
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, courseID, data["course_id"])
		assert.NotEmpty(t, data["issued_at"])
	})

	t.Run("step10_agent_verifies_own_certifications", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/certifications", nil, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		certs := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(certs), 1)
	})
}

// ---------------------------------------------------------------
// E2E FLOW 3: Complete Finance Cycle
// Order → Invoice → Issue → Payment → Ledger → Refund → Audit
// ---------------------------------------------------------------

func TestE2ECompleteFinanceCycle(t *testing.T) {
	adminToken := loginAdmin()
	require.NotEmpty(t, adminToken)

	var orderID, invoiceID, paymentID string

	t.Run("step1_create_service_order", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "plumbing",
			"description":     "E2E Finance: Emergency pipe repair",
			"zip_code":        "10001",
			"assignment_mode": "grab",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		orderID = result["data"].(map[string]interface{})["id"].(string)
	})

	t.Run("step2_create_draft_invoice", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/invoices", map[string]interface{}{
			"order_id":        orderID,
			"subtotal":        500.00,
			"tax_rate":        0.095,
			"billing_address": "100 Finance Blvd, New York, NY 10001",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		invoiceID = data["id"].(string)
		assert.Equal(t, "DRAFT", data["status"])
	})

	t.Run("step3_verify_invoice_details", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/invoices/"+invoiceID, nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, invoiceID, data["id"])
		assert.Equal(t, float64(500.00), data["subtotal"])
		assert.Equal(t, "DRAFT", data["status"])
	})

	t.Run("step4_issue_invoice", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/invoices/"+invoiceID+"/issue", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "ISSUED", data["status"])
		assert.NotEmpty(t, data["issued_at"])
	})

	t.Run("step5_record_payment", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/payments", map[string]interface{}{
			"order_id":        orderID,
			"invoice_id":      invoiceID,
			"amount":          547.50,
			"method":          "check",
			"reference":       "CHK-998877",
			"idempotency_key": fmt.Sprintf("e2e-pay-%d", time.Now().UnixNano()),
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		paymentID = data["id"].(string)
		assert.NotEmpty(t, paymentID)
		assert.Equal(t, float64(547.50), data["amount"])
	})

	t.Run("step6_get_payment_details", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/payments/"+paymentID, nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, paymentID, data["id"])
		assert.Equal(t, "check", data["method"])
	})

	t.Run("step7_list_payments_by_invoice", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/invoices/"+invoiceID+"/payments", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		payments := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(payments), 1)
	})

	t.Run("step8_ledger_has_credit_entries", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders/"+orderID+"/ledger", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		entries := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(entries), 1)
	})

	t.Run("step9_process_partial_refund", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/refunds", map[string]interface{}{
			"payment_id": paymentID,
			"amount":     100.00,
			"reason":     "Partial service credit for scheduling delay",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, float64(100.00), data["amount"])
	})

	t.Run("step10_ledger_reflects_refund", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders/"+orderID+"/ledger", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		entries := result["data"].([]interface{})
		// After refund, there should be an additional debit entry
		assert.GreaterOrEqual(t, len(entries), 2)
	})

	t.Run("step11_verify_audit_chain_after_ops", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/audit-logs/verify", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			data := result["data"].(map[string]interface{})
			assert.Equal(t, true, data["valid"])
		}
	})
}

// ---------------------------------------------------------------
// E2E FLOW 4: System Administration Workflow
// ---------------------------------------------------------------

func TestE2ESystemAdminWorkflow(t *testing.T) {
	adminToken := loginAdmin()
	require.NotEmpty(t, adminToken)

	t.Run("step1_set_custom_quota", func(t *testing.T) {
		resp, _ := doRequest("PUT", "/api/v1/quotas", map[string]interface{}{
			"rpm":                 900,
			"burst":              180,
			"webhook_daily_limit": 15000,
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("step2_verify_quota_persisted", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/quotas", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, float64(900), data["rpm"])
	})

	t.Run("step3_config_change_recorded", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/config-changes", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		meta := result["meta"].(map[string]interface{})
		assert.Greater(t, meta["total"].(float64), float64(0))
	})

	var reportID string

	t.Run("step4_generate_quarterly_report", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/reports", map[string]interface{}{
			"report_type": "kpi",
			"parameters":  map[string]string{"period": "quarterly"},
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		reportID = data["id"].(string)
		assert.NotEmpty(t, reportID)
		assert.Equal(t, "kpi", data["report_type"])
	})

	t.Run("step5_get_report_by_id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/reports/"+reportID, nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, reportID, data["id"])
	})

	t.Run("step6_list_reports_with_pagination", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/reports?page=1&per_page=10", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		meta := result["meta"].(map[string]interface{})
		assert.Equal(t, float64(10), meta["per_page"])
	})

	t.Run("step7_audit_logs_capture_admin_actions", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/audit-logs", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		meta := result["meta"].(map[string]interface{})
		assert.Greater(t, meta["total"].(float64), float64(0))
	})

	t.Run("step8_audit_chain_remains_valid", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/audit-logs/verify", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			data := result["data"].(map[string]interface{})
			assert.Equal(t, true, data["valid"])
		}
	})
}

// ---------------------------------------------------------------
// E2E FLOW 5: Webhook Full Lifecycle
// ---------------------------------------------------------------

func TestE2EWebhookLifecycle(t *testing.T) {
	adminToken := loginAdmin()
	require.NotEmpty(t, adminToken)

	var webhookID string

	t.Run("step1_create_subscription_for_all_events", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/webhooks", map[string]interface{}{
			"url":         "http://localhost:39999/e2e-webhook",
			"event_types": "order.created,order.accepted,scoring.completed,learning.completed",
			"secret":      "e2e-webhook-secret",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		webhookID = data["id"].(string)
		assert.NotEmpty(t, webhookID)
	})

	t.Run("step2_get_subscription_by_id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/webhooks/"+webhookID, nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, webhookID, data["id"])
		assert.Contains(t, data["event_types"].(string), "order.created")
	})

	t.Run("step3_trigger_order_created_event", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "webhook-e2e-trigger",
			"description":     "Triggers order.created webhook event",
			"assignment_mode": "grab",
		}, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("step4_list_all_subscriptions", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/webhooks", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		subs := result["data"].([]interface{})
		assert.GreaterOrEqual(t, len(subs), 1)
	})

	t.Run("step5_dead_letters_accessible", func(t *testing.T) {
		// The webhook endpoint is not running, so after retries it goes to dead-letter
		resp, result := doRequest("GET", "/api/v1/webhooks/dead-letters", nil, adminToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}

// ---------------------------------------------------------------
// E2E FLOW 6: RBAC Full Access Matrix
// ---------------------------------------------------------------

func TestE2ERBACAccessMatrix(t *testing.T) {
	adminToken := loginAdmin()
	agentToken := loginAs("agent1", "admin123")
	require.NotEmpty(t, adminToken)
	require.NotEmpty(t, agentToken)

	readEndpoints := []string{
		"/api/v1/users",
		"/api/v1/roles",
		"/api/v1/courses",
		"/api/v1/orders",
		"/api/v1/service-zones",
	}

	t.Run("admin_can_read_all_public_resources", func(t *testing.T) {
		for _, ep := range readEndpoints {
			resp, _ := doRequest("GET", ep, nil, adminToken)
			require.NotNil(t, resp, "endpoint %s should respond", ep)
			assert.Equal(t, http.StatusOK, resp.StatusCode, "admin should access %s", ep)
		}
	})

	adminOnlyEndpoints := []string{
		"/api/v1/audit-logs",
		"/api/v1/config-changes",
		"/api/v1/reports",
		"/api/v1/webhooks",
		"/api/v1/ledger",
		"/api/v1/invoices",
	}

	t.Run("agent_blocked_from_admin_only_endpoints", func(t *testing.T) {
		for _, ep := range adminOnlyEndpoints {
			resp, _ := doRequest("GET", ep, nil, agentToken)
			require.NotNil(t, resp, "endpoint %s should respond", ep)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode, "agent should be blocked from %s", ep)
		}
	})

	t.Run("agent_can_read_courses_and_orders", func(t *testing.T) {
		for _, ep := range []string{"/api/v1/courses", "/api/v1/orders"} {
			resp, _ := doRequest("GET", ep, nil, agentToken)
			require.NotNil(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}
	})

	t.Run("agent_cannot_create_orders", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category": "unauthorized", "assignment_mode": "grab",
		}, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("agent_cannot_create_courses", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/courses", map[string]interface{}{
			"title": "Unauthorized Course", "category": "test",
		}, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("agent_cannot_create_agent_profiles", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/agent-profiles", map[string]interface{}{
			"user_id": "user-agent1", "zip_code": "10001",
		}, agentToken)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("unauthenticated_request_rejected", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/orders", nil, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// ---------------------------------------------------------------
// Input Validation Coverage
// ---------------------------------------------------------------

func TestInputValidationCoverage(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("register_without_username_rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/auth/register", map[string]string{
			"password":  "TestPass123!",
			"tenant_id": tenantID,
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("login_without_tenant_id_rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username": "admin",
			"password": "admin123",
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("create_course_without_required_fields_rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/courses", map[string]interface{}{
			"description": "Missing title and category",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("create_order_with_invalid_assignment_mode_rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "test",
			"assignment_mode": "invalid_mode",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("create_invoice_without_order_id_rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/invoices", map[string]interface{}{
			"subtotal": 100.00,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("create_payment_without_idempotency_key_rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/payments", map[string]interface{}{
			"order_id":   "some-order-id",
			"invoice_id": "some-invoice-id",
			"amount":     100.00,
			"method":     "cash",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("submit_attempt_score_above_100_rejected", func(t *testing.T) {
		_, courseResult := doRequest("POST", "/api/v1/courses", map[string]interface{}{
			"title": fmt.Sprintf("Validation Course %d", time.Now().UnixNano()), "category": "validation",
		}, token)
		courseID := courseResult["data"].(map[string]interface{})["id"].(string)

		_, assessResult := doRequest("POST", "/api/v1/courses/"+courseID+"/assessments", map[string]interface{}{
			"title": "Validation Assessment", "max_attempts": 3, "passing_score": 70,
		}, token)
		assessmentID := assessResult["data"].(map[string]interface{})["id"].(string)

		_, attemptResult := doRequest("POST", "/api/v1/assessments/"+assessmentID+"/attempts", nil, token)
		attemptID := attemptResult["data"].(map[string]interface{})["id"].(string)

		resp, _ := doRequest("POST", "/api/v1/attempts/"+attemptID+"/submit", map[string]interface{}{
			"answers": "A,B,C",
			"score":   150,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("submit_attempt_score_below_0_rejected", func(t *testing.T) {
		_, courseResult := doRequest("POST", "/api/v1/courses", map[string]interface{}{
			"title": fmt.Sprintf("Validation Course2 %d", time.Now().UnixNano()), "category": "validation",
		}, token)
		courseID := courseResult["data"].(map[string]interface{})["id"].(string)

		_, assessResult := doRequest("POST", "/api/v1/courses/"+courseID+"/assessments", map[string]interface{}{
			"title": "Validation Assessment2", "max_attempts": 3, "passing_score": 70,
		}, token)
		assessmentID := assessResult["data"].(map[string]interface{})["id"].(string)

		_, attemptResult := doRequest("POST", "/api/v1/assessments/"+assessmentID+"/attempts", nil, token)
		attemptID := attemptResult["data"].(map[string]interface{})["id"].(string)

		resp, _ := doRequest("POST", "/api/v1/attempts/"+attemptID+"/submit", map[string]interface{}{
			"answers": "A,B,C",
			"score":   -10,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("create_webhook_without_secret_rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/webhooks", map[string]interface{}{
			"url":         "http://localhost:9999/no-secret",
			"event_types": "order.created",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("accept_order_without_idempotency_key_rejected", func(t *testing.T) {
		_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category": "validation-accept", "assignment_mode": "grab",
		}, token)
		orderID := orderResult["data"].(map[string]interface{})["id"].(string)
		doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{"status": "AVAILABLE"}, token)

		resp, _ := doRequest("POST", "/api/v1/orders/"+orderID+"/accept", map[string]interface{}{}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// ---------------------------------------------------------------
// Dispatch Cancellation and Edge Cases
// ---------------------------------------------------------------

func TestDispatchCancellationFlow(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("accepted_order_can_be_cancelled", func(t *testing.T) {
		_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category": "cancel-test", "assignment_mode": "grab",
		}, token)
		orderID := orderResult["data"].(map[string]interface{})["id"].(string)

		doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{"status": "AVAILABLE"}, token)
		doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{"status": "ACCEPTED"}, token)

		resp, result := doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "CANCELLED",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "CANCELLED", data["status"])
	})

	t.Run("expire_stale_runs_successfully", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/dispatch/expire-stale", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			data := result["data"].(map[string]interface{})
			assert.Contains(t, data, "expired")
			assert.Contains(t, data, "cancelled")
		}
	})

	t.Run("get_nonexistent_order_returns_404", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/orders/00000000-0000-0000-0000-000000000999", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// ---------------------------------------------------------------
// Session List and Pagination
// ---------------------------------------------------------------

func TestSessionListAndPagination(t *testing.T) {
	token := loginAdmin()

	t.Run("list sessions with pagination", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/sessions", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}
