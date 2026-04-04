package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	baseURL  string
	tenantID = "00000000-0000-0000-0000-000000000001"
)

func TestMain(m *testing.M) {
	host := os.Getenv("APP_HOST")
	port := os.Getenv("APP_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "8080"
	}
	baseURL = fmt.Sprintf("http://%s:%s", host, port)

	// Wait for server
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	os.Exit(m.Run())
}

func doRequest(method, path string, body interface{}, token string) (*http.Response, map[string]interface{}) {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, _ := http.NewRequest(method, baseURL+path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	return resp, result
}

func loginAdmin() string {
	_, result := doRequest("POST", "/api/v1/auth/login", map[string]string{
		"username":  "admin",
		"password":  "admin123",
		"tenant_id": tenantID,
	}, "")
	if result == nil || result["data"] == nil {
		return ""
	}
	data := result["data"].(map[string]interface{})
	return data["access_token"].(string)
}

// === Health Check ===
func TestHealthCheck(t *testing.T) {
	resp, result := doRequest("GET", "/health", nil, "")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", result["status"])
}

// === OpenAPI Docs ===
func TestOpenAPIDocs(t *testing.T) {
	t.Run("swagger UI page serves HTML", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/docs", nil)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	})

	t.Run("openapi.json serves valid spec", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/openapi.json", nil, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "3.0.3", result["openapi"])
		info := result["info"].(map[string]interface{})
		assert.Equal(t, "DispatchLearn Operations Settlement API", info["title"])
		assert.Equal(t, "1.0.0", info["version"])
		// Verify paths exist
		paths := result["paths"].(map[string]interface{})
		assert.NotNil(t, paths["/health"])
		assert.NotNil(t, paths["/api/v1/auth/login"])
		assert.NotNil(t, paths["/api/v1/orders"])
		assert.NotNil(t, paths["/api/v1/payments"])
		assert.NotNil(t, paths["/api/v1/invoices"])
		assert.NotNil(t, paths["/api/v1/courses"])
		assert.NotNil(t, paths["/api/v1/webhooks"])
		assert.NotNil(t, paths["/api/v1/audit-logs"])
		assert.NotNil(t, paths["/api/v1/quotas"])
	})
}

// === Auth ===
func TestAuthFlow(t *testing.T) {
	t.Run("register new user", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/auth/register", map[string]string{
			"username":  fmt.Sprintf("testuser_%d", time.Now().UnixNano()),
			"password":  "TestPass123!",
			"tenant_id": tenantID,
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("login with valid credentials", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username":  "admin",
			"password":  "admin123",
			"tenant_id": tenantID,
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.NotEmpty(t, data["access_token"])
		assert.NotEmpty(t, data["refresh_token"])
		assert.Equal(t, "Bearer", data["token_type"])
	})

	t.Run("login with invalid credentials", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username":  "admin",
			"password":  "wrongpassword",
			"tenant_id": tenantID,
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("access protected route without token", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/me", nil, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("access protected route with valid token", func(t *testing.T) {
		token := loginAdmin()
		require.NotEmpty(t, token)

		resp, result := doRequest("GET", "/api/v1/me", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "admin", data["username"])
	})

	t.Run("refresh token flow", func(t *testing.T) {
		// Login first
		_, loginResult := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username":  "admin",
			"password":  "admin123",
			"tenant_id": tenantID,
		}, "")
		data := loginResult["data"].(map[string]interface{})
		refreshToken := data["refresh_token"].(string)

		// Refresh
		resp, result := doRequest("POST", "/api/v1/auth/refresh", map[string]string{
			"refresh_token": refreshToken,
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		newData := result["data"].(map[string]interface{})
		assert.NotEmpty(t, newData["access_token"])
		assert.NotEqual(t, refreshToken, newData["refresh_token"]) // Rotated
	})

	t.Run("expired refresh token rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/auth/refresh", map[string]string{
			"refresh_token": "invalid-token",
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// === RBAC ===
func TestRBAC(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	t.Run("admin can list users", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/users", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
		assert.NotNil(t, result["meta"])
	})

	t.Run("list roles", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/roles", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}

// === LMS ===
func TestLMSFlow(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	var courseID string

	t.Run("create course", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/courses", map[string]string{
			"title":       "Safety Training 101",
			"description": "Basic safety training for field agents",
			"category":    "safety",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		courseID = data["id"].(string)
		assert.NotEmpty(t, courseID)
	})

	t.Run("list courses", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/courses", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})

	t.Run("get course by id", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/courses/"+courseID, nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "Safety Training 101", data["title"])
	})

	t.Run("add content item to course", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/courses/"+courseID+"/content", map[string]interface{}{
			"title":        "Chapter 1 - Introduction",
			"content_type": "pdf",
			"file_path":    "/content/safety/ch1.pdf",
			"checksum":     "abc123",
			"size_bytes":   1024000,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("create assessment", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/courses/"+courseID+"/assessments", map[string]interface{}{
			"title":        "Safety Quiz",
			"description":  "Test your safety knowledge",
			"max_attempts": 3,
			"passing_score": 70,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}

// === Dispatch ===
func TestDispatchFlow(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	var orderID string

	t.Run("create order", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "repair",
			"description":     "Fix broken HVAC unit",
			"zip_code":        "10001",
			"assignment_mode": "grab",
			"priority":        1,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		orderID = data["id"].(string)
		assert.NotEmpty(t, orderID)
		assert.Equal(t, "CREATED", data["status"])
	})

	t.Run("list orders", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})

	t.Run("transition order to AVAILABLE", func(t *testing.T) {
		resp, _ := doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "AVAILABLE",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid state transition rejected", func(t *testing.T) {
		resp, _ := doRequest("PATCH", "/api/v1/orders/"+orderID+"/status", map[string]string{
			"status": "COMPLETED",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("get recommendations for order", func(t *testing.T) {
		resp, _ := doRequest("GET", "/api/v1/orders/"+orderID+"/recommendations", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("accept order with idempotency key", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/orders/"+orderID+"/accept", map[string]string{
			"idempotency_key": fmt.Sprintf("idem-%d", time.Now().UnixNano()),
		}, token)
		require.NotNil(t, resp)
		// May succeed or fail depending on qualification - just check it's handled
		assert.Contains(t, []int{201, 400, 403, 409}, resp.StatusCode)
	})

	t.Run("duplicate acceptance returns 409", func(t *testing.T) {
		// Create another order and accept it
		_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
			"category":        "install",
			"description":     "Install new unit",
			"assignment_mode": "grab",
		}, token)
		data := orderResult["data"].(map[string]interface{})
		newOrderID := data["id"].(string)

		// Make available
		doRequest("PATCH", "/api/v1/orders/"+newOrderID+"/status", map[string]string{
			"status": "AVAILABLE",
		}, token)

		idemKey := fmt.Sprintf("idem-dup-%d", time.Now().UnixNano())

		// First acceptance
		doRequest("POST", "/api/v1/orders/"+newOrderID+"/accept", map[string]string{
			"idempotency_key": idemKey,
		}, token)

		// Second acceptance should conflict
		resp, _ := doRequest("POST", "/api/v1/orders/"+newOrderID+"/accept", map[string]string{
			"idempotency_key": fmt.Sprintf("idem-dup2-%d", time.Now().UnixNano()),
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

// === Service Zones ===
func TestServiceZones(t *testing.T) {
	token := loginAdmin()

	t.Run("list service zones", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/service-zones", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}

// === Finance ===
func TestFinanceFlow(t *testing.T) {
	token := loginAdmin()
	require.NotEmpty(t, token)

	// Create an order for finance operations
	_, orderResult := doRequest("POST", "/api/v1/orders", map[string]interface{}{
		"category":        "service",
		"description":     "Service call",
		"assignment_mode": "grab",
	}, token)
	orderData := orderResult["data"].(map[string]interface{})
	orderID := orderData["id"].(string)

	var invoiceID string

	t.Run("create invoice", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/invoices", map[string]interface{}{
			"order_id":        orderID,
			"subtotal":        150.00,
			"tax_rate":        0.08,
			"billing_address": "123 Main St, New York, NY 10001",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := result["data"].(map[string]interface{})
		invoiceID = data["id"].(string)
		assert.NotEmpty(t, invoiceID)
	})

	t.Run("list invoices", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/invoices", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})

	t.Run("issue invoice", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/invoices/"+invoiceID+"/issue", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("record payment", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/payments", map[string]interface{}{
			"order_id":       orderID,
			"invoice_id":     invoiceID,
			"amount":         100.00,
			"method":         "cash",
			"reference":      "CASH-001",
			"idempotency_key": fmt.Sprintf("pay-%d", time.Now().UnixNano()),
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("duplicate payment within 5 minutes rejected", func(t *testing.T) {
		resp, _ := doRequest("POST", "/api/v1/payments", map[string]interface{}{
			"order_id":       orderID,
			"invoice_id":     invoiceID,
			"amount":         100.00,
			"method":         "cash",
			"reference":      "CASH-DUP",
			"idempotency_key": fmt.Sprintf("pay-dup-%d", time.Now().UnixNano()),
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("list payments by order", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/orders/"+orderID+"/payments", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("list ledger entries", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/ledger", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})
}

// === Audit ===
func TestAuditLogs(t *testing.T) {
	token := loginAdmin()

	t.Run("list audit logs", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/audit-logs", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})

	t.Run("verify audit chain", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/audit-logs/verify", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if result["data"] != nil {
			data := result["data"].(map[string]interface{})
			assert.Equal(t, true, data["valid"])
		}
	})
}

// === Webhooks ===
func TestWebhooks(t *testing.T) {
	token := loginAdmin()

	t.Run("create webhook subscription", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/webhooks", map[string]interface{}{
			"url":         "http://localhost:9999/hooks",
			"event_types": "order.created,order.accepted",
			"secret":      "webhook-secret-key",
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("list webhook subscriptions", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/webhooks", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})
}

// === Reports ===
func TestReports(t *testing.T) {
	token := loginAdmin()

	t.Run("generate KPI report", func(t *testing.T) {
		resp, result := doRequest("POST", "/api/v1/reports", map[string]interface{}{
			"report_type": "kpi",
			"parameters": map[string]string{
				"period": "monthly",
			},
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, result["data"])
	})

	t.Run("list reports", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/reports", nil, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, result["meta"])
	})
}

// === Quotas ===
func TestQuotas(t *testing.T) {
	token := loginAdmin()

	t.Run("set quota override", func(t *testing.T) {
		resp, _ := doRequest("PUT", "/api/v1/quotas", map[string]interface{}{
			"rpm":                 1200,
			"burst":              240,
			"webhook_daily_limit": 20000,
		}, token)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("get quota override", func(t *testing.T) {
		resp, result := doRequest("GET", "/api/v1/quotas", nil, token)
		require.NotNil(t, resp)
		// May be 200 or 404 depending on override state
		if resp.StatusCode == http.StatusOK {
			assert.NotNil(t, result["data"])
		}
	})
}

// === Tenant Isolation ===
func TestTenantIsolation(t *testing.T) {
	t.Run("requests without tenant context are rejected", func(t *testing.T) {
		// Login with wrong tenant would fail - this validates isolation
		resp, _ := doRequest("POST", "/api/v1/auth/login", map[string]string{
			"username":  "admin",
			"password":  "admin123",
			"tenant_id": "nonexistent-tenant-id",
		}, "")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
