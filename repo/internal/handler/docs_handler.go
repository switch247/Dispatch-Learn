package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SwaggerUIHTML serves a fully offline Swagger UI page.
// Assets are served from /static/swagger/ which is populated during Docker build.
// Falls back to CDN if local assets are not found (dev mode).
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>DispatchLearn API Documentation</title>
  <link rel="stylesheet" href="/static/swagger/swagger-ui.css"
        onerror="this.href='https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css'">
  <style>
    html { box-sizing: border-box; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script>
    // Try local first, fall back to CDN
    function loadScript(src, fallback) {
      var s = document.createElement('script');
      s.src = src;
      s.onerror = function() {
        var f = document.createElement('script');
        f.src = fallback;
        f.onload = initSwagger;
        document.body.appendChild(f);
      };
      s.onload = initSwagger;
      document.body.appendChild(s);
    }
    function initSwagger() {
      if (typeof SwaggerUIBundle !== 'undefined') {
        SwaggerUIBundle({
          url: "/api/v1/openapi.json",
          dom_id: '#swagger-ui',
          deepLinking: true,
          presets: [
            SwaggerUIBundle.presets.apis,
            SwaggerUIBundle.SwaggerUIStandalonePreset
          ],
          layout: "BaseLayout"
        });
      }
    }
    loadScript('/static/swagger/swagger-ui-bundle.js',
               'https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js');
  </script>
</body>
</html>`

func DocsHandler(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerUIHTML))
}

func OpenAPISpecHandler(c *gin.Context) {
	c.Data(http.StatusOK, "application/json", []byte(openAPISpec))
}

const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "DispatchLearn Operations Settlement API",
    "description": "Production-grade, offline-first field service + LMS + settlement platform. Combines dispatch operations, workforce learning management, and settlement/finance logging with full multi-tenant isolation, RBAC, tamper-evident audit logging, and encrypted sensitive data.",
    "version": "1.0.0",
    "contact": {
      "name": "DispatchLearn API Support"
    }
  },
  "servers": [
    {
      "url": "http://localhost:8080",
      "description": "Local development"
    }
  ],
  "tags": [
    {"name": "Health", "description": "Service health check"},
    {"name": "Auth", "description": "Authentication - login, register, token refresh"},
    {"name": "Users", "description": "User management and role assignment"},
    {"name": "Sessions", "description": "Session management and revocation"},
    {"name": "Courses", "description": "LMS course management"},
    {"name": "Assessments", "description": "Assessment creation and grading"},
    {"name": "Certifications", "description": "Certification issuance and listing"},
    {"name": "Reader Artifacts", "description": "Bookmarks, highlights, and annotations"},
    {"name": "Orders", "description": "Service order lifecycle and dispatch"},
    {"name": "Dispatch", "description": "Order acceptance, recommendations, and expiration"},
    {"name": "Service Zones", "description": "Geographic service zone management"},
    {"name": "Agent Profiles", "description": "Field agent profiles and availability"},
    {"name": "Invoices", "description": "Invoice creation and issuance"},
    {"name": "Payments", "description": "Payment recording and duplicate prevention"},
    {"name": "Refunds", "description": "Refund processing via ledger reversals"},
    {"name": "Ledger", "description": "Financial ledger entries"},
    {"name": "Audit", "description": "Tamper-evident audit logs and chain verification"},
    {"name": "Config", "description": "Configuration change tracking"},
    {"name": "Reports", "description": "KPI report generation and export"},
    {"name": "Webhooks", "description": "Webhook subscription and delivery management"},
    {"name": "Quotas", "description": "Per-tenant rate limit and quota overrides"}
  ],
  "components": {
    "securitySchemes": {
      "BearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT",
        "description": "JWT access token (30-minute expiry). Obtain via POST /api/v1/auth/login"
      }
    },
    "schemas": {
      "APIResponse": {
        "type": "object",
        "properties": {
          "data": {"description": "Response payload"},
          "meta": {"$ref": "#/components/schemas/Meta"},
          "errors": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/APIError"}
          }
        }
      },
      "Meta": {
        "type": "object",
        "properties": {
          "page": {"type": "integer"},
          "per_page": {"type": "integer"},
          "total": {"type": "integer"},
          "total_pages": {"type": "integer"}
        }
      },
      "APIError": {
        "type": "object",
        "properties": {
          "code": {"type": "string"},
          "message": {"type": "string"},
          "field": {"type": "string"}
        }
      },
      "LoginRequest": {
        "type": "object",
        "required": ["username", "password", "tenant_id"],
        "properties": {
          "username": {"type": "string", "example": "admin"},
          "password": {"type": "string", "example": "admin123"},
          "tenant_id": {"type": "string", "format": "uuid", "example": "00000000-0000-0000-0000-000000000001"}
        }
      },
      "LoginResponse": {
        "type": "object",
        "properties": {
          "access_token": {"type": "string"},
          "refresh_token": {"type": "string"},
          "expires_in": {"type": "integer", "example": 1800},
          "token_type": {"type": "string", "example": "Bearer"}
        }
      },
      "RefreshRequest": {
        "type": "object",
        "required": ["refresh_token"],
        "properties": {
          "refresh_token": {"type": "string"}
        }
      },
      "User": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "tenant_id": {"type": "string", "format": "uuid"},
          "username": {"type": "string"},
          "email": {"type": "string"},
          "full_name": {"type": "string"},
          "is_active": {"type": "boolean"},
          "last_login_at": {"type": "string", "format": "date-time", "nullable": true},
          "roles": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/Role"}
          },
          "created_at": {"type": "string", "format": "date-time"},
          "updated_at": {"type": "string", "format": "date-time"}
        }
      },
      "Role": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "tenant_id": {"type": "string", "format": "uuid"},
          "name": {"type": "string", "example": "admin"},
          "description": {"type": "string"},
          "permissions": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/Permission"}
          }
        }
      },
      "Permission": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "name": {"type": "string", "example": "orders:write"},
          "resource": {"type": "string"},
          "action": {"type": "string"}
        }
      },
      "UserSession": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "user_id": {"type": "string", "format": "uuid"},
          "user_agent": {"type": "string"},
          "ip_address": {"type": "string"},
          "expires_at": {"type": "string", "format": "date-time"},
          "is_active": {"type": "boolean"},
          "created_at": {"type": "string", "format": "date-time"}
        }
      },
      "CreateCourseRequest": {
        "type": "object",
        "required": ["title", "category"],
        "properties": {
          "title": {"type": "string", "example": "Safety Training 101"},
          "description": {"type": "string"},
          "category": {"type": "string", "example": "safety"}
        }
      },
      "Course": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "tenant_id": {"type": "string", "format": "uuid"},
          "title": {"type": "string"},
          "description": {"type": "string"},
          "category": {"type": "string"},
          "is_active": {"type": "boolean"},
          "content_items": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/ContentItem"}
          },
          "assessments": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/Assessment"}
          }
        }
      },
      "ContentItem": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "course_id": {"type": "string", "format": "uuid"},
          "title": {"type": "string"},
          "content_type": {"type": "string", "enum": ["epub", "pdf", "html"]},
          "file_path": {"type": "string"},
          "checksum": {"type": "string"},
          "size_bytes": {"type": "integer", "description": "Max 50MB (52428800)"},
          "sort_order": {"type": "integer"}
        }
      },
      "Assessment": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "course_id": {"type": "string", "format": "uuid"},
          "title": {"type": "string"},
          "description": {"type": "string"},
          "max_attempts": {"type": "integer", "default": 3},
          "passing_score": {"type": "integer", "default": 70},
          "is_active": {"type": "boolean"}
        }
      },
      "AssessmentAttempt": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "assessment_id": {"type": "string", "format": "uuid"},
          "user_id": {"type": "string", "format": "uuid"},
          "attempt_no": {"type": "integer"},
          "started_at": {"type": "string", "format": "date-time"},
          "completed_at": {"type": "string", "format": "date-time", "nullable": true}
        }
      },
      "SubmitAttemptRequest": {
        "type": "object",
        "required": ["answers", "score"],
        "properties": {
          "answers": {"type": "string"},
          "score": {"type": "integer", "minimum": 0, "maximum": 100}
        }
      },
      "Grade": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "user_id": {"type": "string", "format": "uuid"},
          "assessment_id": {"type": "string", "format": "uuid"},
          "attempt_id": {"type": "string", "format": "uuid"},
          "numeric_score": {"type": "string", "description": "Encrypted at rest"},
          "letter_grade": {"type": "string", "example": "B+"},
          "is_passing": {"type": "boolean"},
          "graded_at": {"type": "string", "format": "date-time"}
        }
      },
      "Certification": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "user_id": {"type": "string", "format": "uuid"},
          "course_id": {"type": "string", "format": "uuid"},
          "issued_at": {"type": "string", "format": "date-time"},
          "expires_at": {"type": "string", "format": "date-time"},
          "is_revoked": {"type": "boolean"}
        }
      },
      "IssueCertificationRequest": {
        "type": "object",
        "required": ["user_id", "course_id"],
        "properties": {
          "user_id": {"type": "string", "format": "uuid"},
          "course_id": {"type": "string", "format": "uuid"}
        }
      },
      "ReaderArtifact": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "user_id": {"type": "string", "format": "uuid"},
          "content_id": {"type": "string", "format": "uuid"},
          "artifact_type": {"type": "string", "enum": ["bookmark", "highlight", "annotation"]},
          "position": {"type": "string"},
          "content": {"type": "string"}
        }
      },
      "CreateOrderRequest": {
        "type": "object",
        "required": ["category"],
        "properties": {
          "category": {"type": "string", "example": "repair"},
          "description": {"type": "string"},
          "zone_id": {"type": "string", "format": "uuid"},
          "address": {"type": "string", "description": "Encrypted at rest"},
          "zip_code": {"type": "string", "example": "10001"},
          "time_window_start": {"type": "string", "format": "date-time"},
          "time_window_end": {"type": "string", "format": "date-time"},
          "assignment_mode": {"type": "string", "enum": ["grab", "assigned"], "default": "grab"},
          "priority": {"type": "integer", "default": 0}
        }
      },
      "Order": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "tenant_id": {"type": "string", "format": "uuid"},
          "order_no": {"type": "string"},
          "category": {"type": "string"},
          "description": {"type": "string"},
          "status": {"type": "string", "enum": ["CREATED", "AVAILABLE", "ACCEPTED", "IN_PROGRESS", "COMPLETED", "EXPIRED", "CANCELLED"]},
          "zone_id": {"type": "string", "format": "uuid"},
          "address": {"type": "string", "description": "Encrypted at rest"},
          "zip_code": {"type": "string"},
          "time_window_start": {"type": "string", "format": "date-time", "nullable": true},
          "time_window_end": {"type": "string", "format": "date-time", "nullable": true},
          "assigned_agent_id": {"type": "string", "format": "uuid", "nullable": true},
          "assignment_mode": {"type": "string", "enum": ["grab", "assigned"]},
          "priority": {"type": "integer"},
          "available_at": {"type": "string", "format": "date-time", "nullable": true},
          "accepted_at": {"type": "string", "format": "date-time", "nullable": true},
          "completed_at": {"type": "string", "format": "date-time", "nullable": true},
          "created_at": {"type": "string", "format": "date-time"},
          "updated_at": {"type": "string", "format": "date-time"}
        }
      },
      "AcceptOrderRequest": {
        "type": "object",
        "required": ["idempotency_key"],
        "properties": {
          "idempotency_key": {"type": "string", "description": "Unique key to prevent duplicate acceptances"}
        }
      },
      "DispatchAcceptance": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "order_id": {"type": "string", "format": "uuid"},
          "agent_id": {"type": "string", "format": "uuid"},
          "idempotency_key": {"type": "string"},
          "accepted_at": {"type": "string", "format": "date-time"}
        }
      },
      "Recommendation": {
        "type": "object",
        "properties": {
          "agent_id": {"type": "string", "format": "uuid"},
          "user_id": {"type": "string", "format": "uuid"},
          "full_name": {"type": "string"},
          "distance_km": {"type": "number", "format": "double"},
          "reputation_score": {"type": "number", "format": "double"},
          "open_tasks": {"type": "integer"},
          "ranking_score": {"type": "number", "format": "double", "description": "50% distance + 30% reputation + 20% workload"},
          "is_qualified": {"type": "boolean"}
        }
      },
      "TransitionRequest": {
        "type": "object",
        "required": ["status"],
        "properties": {
          "status": {"type": "string", "enum": ["AVAILABLE", "ACCEPTED", "IN_PROGRESS", "COMPLETED", "EXPIRED", "CANCELLED"]}
        }
      },
      "ServiceZone": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "name": {"type": "string"},
          "zip_codes": {"type": "string", "description": "Comma-separated"},
          "centroid_lat": {"type": "number", "format": "double"},
          "centroid_lng": {"type": "number", "format": "double"}
        }
      },
      "AgentProfile": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "user_id": {"type": "string", "format": "uuid"},
          "zone_id": {"type": "string", "format": "uuid"},
          "zip_code": {"type": "string"},
          "is_available": {"type": "boolean"},
          "max_workload": {"type": "integer", "default": 8},
          "reputation_score": {"type": "number", "format": "double"}
        }
      },
      "CreateInvoiceRequest": {
        "type": "object",
        "required": ["order_id", "subtotal"],
        "properties": {
          "order_id": {"type": "string", "format": "uuid"},
          "subtotal": {"type": "number", "format": "double", "example": 150.00},
          "tax_rate": {"type": "number", "format": "double", "example": 0.08},
          "billing_address": {"type": "string", "description": "Encrypted at rest"}
        }
      },
      "Invoice": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "order_id": {"type": "string", "format": "uuid"},
          "invoice_no": {"type": "string"},
          "status": {"type": "string", "enum": ["DRAFT", "ISSUED", "PAID", "PARTIAL", "VOIDED"]},
          "subtotal": {"type": "number", "format": "double"},
          "tax_rate": {"type": "number", "format": "double"},
          "tax_amount": {"type": "number", "format": "double"},
          "total_amount": {"type": "number", "format": "double"},
          "billing_address": {"type": "string", "description": "Encrypted at rest"},
          "issued_at": {"type": "string", "format": "date-time", "nullable": true},
          "due_at": {"type": "string", "format": "date-time", "nullable": true}
        }
      },
      "CreatePaymentRequest": {
        "type": "object",
        "required": ["order_id", "invoice_id", "amount", "method", "idempotency_key"],
        "properties": {
          "order_id": {"type": "string", "format": "uuid"},
          "invoice_id": {"type": "string", "format": "uuid"},
          "amount": {"type": "number", "format": "double", "example": 100.00},
          "method": {"type": "string", "enum": ["cash", "check", "card_present", "house_account"]},
          "reference": {"type": "string", "description": "Encrypted at rest"},
          "idempotency_key": {"type": "string", "description": "Required for duplicate prevention"}
        }
      },
      "Payment": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "order_id": {"type": "string", "format": "uuid"},
          "invoice_id": {"type": "string", "format": "uuid"},
          "amount": {"type": "number", "format": "double"},
          "method": {"type": "string", "enum": ["cash", "check", "card_present", "house_account"]},
          "reference": {"type": "string", "description": "Encrypted at rest"},
          "idempotency_key": {"type": "string"},
          "status": {"type": "string"},
          "processed_at": {"type": "string", "format": "date-time"}
        }
      },
      "CreateRefundRequest": {
        "type": "object",
        "required": ["payment_id", "amount", "reason"],
        "properties": {
          "payment_id": {"type": "string", "format": "uuid"},
          "amount": {"type": "number", "format": "double"},
          "reason": {"type": "string"}
        }
      },
      "LedgerEntry": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "order_id": {"type": "string", "format": "uuid"},
          "invoice_id": {"type": "string", "format": "uuid"},
          "payment_id": {"type": "string", "format": "uuid"},
          "entry_type": {"type": "string", "enum": ["debit", "credit"]},
          "amount": {"type": "number", "format": "double"},
          "description": {"type": "string"},
          "balance_after": {"type": "number", "format": "double"}
        }
      },
      "AuditLog": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "tenant_id": {"type": "string", "format": "uuid"},
          "actor_id": {"type": "string", "format": "uuid"},
          "action": {"type": "string"},
          "entity_type": {"type": "string"},
          "entity_id": {"type": "string", "format": "uuid"},
          "before_state": {"type": "string", "description": "JSON of previous state"},
          "after_state": {"type": "string", "description": "JSON of new state"},
          "timestamp": {"type": "string", "format": "date-time"},
          "previous_hash": {"type": "string", "description": "SHA-256 hash of previous log entry"},
          "current_hash": {"type": "string", "description": "SHA-256 hash chain: hash(prev_hash + current_record)"},
          "ip_address": {"type": "string"}
        }
      },
      "ConfigChange": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "changed_by": {"type": "string", "format": "uuid"},
          "config_key": {"type": "string"},
          "old_value": {"type": "string"},
          "new_value": {"type": "string"},
          "reason": {"type": "string"}
        }
      },
      "Report": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "name": {"type": "string"},
          "report_type": {"type": "string"},
          "parameters": {"type": "string"},
          "file_path": {"type": "string"},
          "file_checksum": {"type": "string", "description": "SHA-256 checksum of exported file"},
          "generated_at": {"type": "string", "format": "date-time"},
          "generated_by": {"type": "string", "format": "uuid"},
          "status": {"type": "string", "enum": ["pending", "generating", "completed", "failed"]}
        }
      },
      "GenerateReportRequest": {
        "type": "object",
        "required": ["report_type"],
        "properties": {
          "report_type": {"type": "string", "example": "kpi"},
          "parameters": {"type": "object", "additionalProperties": {"type": "string"}}
        }
      },
      "CreateWebhookRequest": {
        "type": "object",
        "required": ["url", "event_types", "secret"],
        "properties": {
          "url": {"type": "string", "format": "uri", "example": "http://192.168.1.50:9000/hooks"},
          "event_types": {"type": "string", "description": "Comma-separated event types", "example": "order.created,order.accepted,grade.recorded"},
          "secret": {"type": "string", "description": "HMAC-SHA256 signing secret"},
          "date_range_start": {"type": "string", "format": "date-time"},
          "date_range_end": {"type": "string", "format": "date-time"}
        }
      },
      "WebhookSubscription": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "url": {"type": "string"},
          "event_types": {"type": "string"},
          "is_active": {"type": "boolean"},
          "date_range_start": {"type": "string", "format": "date-time", "nullable": true},
          "date_range_end": {"type": "string", "format": "date-time", "nullable": true}
        }
      },
      "WebhookDelivery": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "subscription_id": {"type": "string", "format": "uuid"},
          "delivery_id": {"type": "string", "description": "Replay-safe unique delivery identifier"},
          "event_type": {"type": "string"},
          "payload": {"type": "string"},
          "response_code": {"type": "integer"},
          "attempt_count": {"type": "integer"},
          "max_attempts": {"type": "integer", "default": 5},
          "status": {"type": "string", "enum": ["pending", "delivered", "failed", "dead_letter"]},
          "nonce": {"type": "string", "description": "Anti-replay nonce"}
        }
      },
      "QuotaOverride": {
        "type": "object",
        "properties": {
          "rpm": {"type": "integer", "default": 600, "description": "Requests per minute"},
          "burst": {"type": "integer", "default": 120},
          "webhook_daily_limit": {"type": "integer", "default": 10000}
        }
      },
      "AssignRoleRequest": {
        "type": "object",
        "required": ["role"],
        "properties": {
          "role": {"type": "string", "example": "dispatcher"}
        }
      }
    },
    "parameters": {
      "PageParam": {
        "name": "page",
        "in": "query",
        "schema": {"type": "integer", "default": 1, "minimum": 1}
      },
      "PerPageParam": {
        "name": "per_page",
        "in": "query",
        "schema": {"type": "integer", "default": 20, "minimum": 1, "maximum": 100}
      }
    },
    "responses": {
      "Unauthorized": {
        "description": "Missing or invalid JWT token",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/APIResponse"}
          }
        }
      },
      "Forbidden": {
        "description": "Insufficient role/permissions or tenant violation",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/APIResponse"}
          }
        }
      },
      "NotFound": {
        "description": "Resource not found",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/APIResponse"}
          }
        }
      },
      "ValidationError": {
        "description": "Request validation failed (422)",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/APIResponse"}
          }
        }
      },
      "Conflict": {
        "description": "Resource conflict (duplicate acceptance, duplicate payment)",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/APIResponse"}
          }
        }
      },
      "TooManyRequests": {
        "description": "Rate limit exceeded (600 req/min default)",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/APIResponse"}
          }
        }
      }
    }
  },
  "paths": {
    "/health": {
      "get": {
        "tags": ["Health"],
        "summary": "Service health check",
        "operationId": "healthCheck",
        "responses": {
          "200": {
            "description": "Service is healthy",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "status": {"type": "string", "example": "ok"},
                    "service": {"type": "string", "example": "dispatchlearn"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/v1/auth/register": {
      "post": {
        "tags": ["Auth"],
        "summary": "Register a new user",
        "operationId": "register",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/LoginRequest"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "User created",
            "content": {
              "application/json": {
                "schema": {
                  "allOf": [{"$ref": "#/components/schemas/APIResponse"}],
                  "properties": {"data": {"$ref": "#/components/schemas/User"}}
                }
              }
            }
          },
          "400": {"description": "Registration failed"},
          "422": {"$ref": "#/components/responses/ValidationError"}
        }
      }
    },
    "/api/v1/auth/login": {
      "post": {
        "tags": ["Auth"],
        "summary": "Authenticate and obtain JWT tokens",
        "description": "Returns a 30-minute access token and a rotating refresh token. Max 10 active sessions per user; oldest is revoked when cap exceeded. Account locks after 5 failed attempts for 30 minutes.",
        "operationId": "login",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/LoginRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Login successful",
            "content": {
              "application/json": {
                "schema": {
                  "allOf": [{"$ref": "#/components/schemas/APIResponse"}],
                  "properties": {"data": {"$ref": "#/components/schemas/LoginResponse"}}
                }
              }
            }
          },
          "401": {"description": "Invalid credentials or account locked"}
        }
      }
    },
    "/api/v1/auth/refresh": {
      "post": {
        "tags": ["Auth"],
        "summary": "Refresh access token using rotating refresh token",
        "description": "Previous refresh token is revoked (rotation). Returns new access + refresh token pair.",
        "operationId": "refreshToken",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/RefreshRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Token refreshed",
            "content": {
              "application/json": {
                "schema": {
                  "allOf": [{"$ref": "#/components/schemas/APIResponse"}],
                  "properties": {"data": {"$ref": "#/components/schemas/LoginResponse"}}
                }
              }
            }
          },
          "401": {"description": "Invalid or expired refresh token"}
        }
      }
    },
    "/api/v1/me": {
      "get": {
        "tags": ["Users"],
        "summary": "Get current authenticated user profile",
        "operationId": "getMe",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {
            "description": "Current user",
            "content": {
              "application/json": {
                "schema": {
                  "allOf": [{"$ref": "#/components/schemas/APIResponse"}],
                  "properties": {"data": {"$ref": "#/components/schemas/User"}}
                }
              }
            }
          },
          "401": {"$ref": "#/components/responses/Unauthorized"}
        }
      }
    },
    "/api/v1/users": {
      "get": {
        "tags": ["Users"],
        "summary": "List users (admin only)",
        "operationId": "listUsers",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"}
        ],
        "responses": {
          "200": {
            "description": "Paginated user list",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/APIResponse"}
              }
            }
          },
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "403": {"$ref": "#/components/responses/Forbidden"}
        }
      }
    },
    "/api/v1/users/{id}": {
      "get": {
        "tags": ["Users"],
        "summary": "Get user by ID (admin only)",
        "operationId": "getUser",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "User found"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/users/{id}/roles": {
      "post": {
        "tags": ["Users"],
        "summary": "Assign role to user (admin only)",
        "description": "Privilege escalation is audit-logged. Available roles: admin, dispatcher, agent, instructor, finance.",
        "operationId": "assignRole",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AssignRoleRequest"}
            }
          }
        },
        "responses": {
          "200": {"description": "Role assigned"},
          "400": {"description": "Assignment failed"},
          "403": {"$ref": "#/components/responses/Forbidden"}
        }
      }
    },
    "/api/v1/roles": {
      "get": {
        "tags": ["Users"],
        "summary": "List available roles",
        "operationId": "listRoles",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {"description": "Role list with permissions"}
        }
      }
    },
    "/api/v1/sessions": {
      "get": {
        "tags": ["Sessions"],
        "summary": "List active sessions for current user",
        "operationId": "listSessions",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {"description": "Active sessions"}
        }
      }
    },
    "/api/v1/sessions/{session_id}": {
      "delete": {
        "tags": ["Sessions"],
        "summary": "Revoke a specific session",
        "operationId": "revokeSession",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "session_id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Session revoked"}
        }
      }
    },
    "/api/v1/courses": {
      "get": {
        "tags": ["Courses"],
        "summary": "List courses",
        "operationId": "listCourses",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"}
        ],
        "responses": {
          "200": {"description": "Paginated course list"}
        }
      },
      "post": {
        "tags": ["Courses"],
        "summary": "Create a course (admin/instructor)",
        "operationId": "createCourse",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CreateCourseRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Course created"},
          "403": {"$ref": "#/components/responses/Forbidden"}
        }
      }
    },
    "/api/v1/courses/{id}": {
      "get": {
        "tags": ["Courses"],
        "summary": "Get course with content items and assessments",
        "operationId": "getCourse",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Course details"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/courses/{id}/content": {
      "post": {
        "tags": ["Courses"],
        "summary": "Add content item to course (admin/instructor)",
        "description": "Supports epub, pdf, html. Max 50MB per item.",
        "operationId": "addContentItem",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/ContentItem"}
            }
          }
        },
        "responses": {
          "201": {"description": "Content added"},
          "400": {"description": "Content exceeds 50MB or invalid type"}
        }
      }
    },
    "/api/v1/courses/{id}/assessments": {
      "post": {
        "tags": ["Assessments"],
        "summary": "Create assessment for course (admin/instructor)",
        "operationId": "createAssessment",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/Assessment"}
            }
          }
        },
        "responses": {
          "201": {"description": "Assessment created"}
        }
      }
    },
    "/api/v1/assessments/{assessment_id}": {
      "get": {
        "tags": ["Assessments"],
        "summary": "Get assessment details",
        "operationId": "getAssessment",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "assessment_id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Assessment details"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/assessments/{assessment_id}/attempts": {
      "post": {
        "tags": ["Assessments"],
        "summary": "Start an assessment attempt",
        "description": "Max 3 attempts per assessment per user (configurable). Returns attempt ID for submission.",
        "operationId": "startAttempt",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "assessment_id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "201": {"description": "Attempt started"},
          "400": {"description": "Max attempts exceeded"}
        }
      }
    },
    "/api/v1/attempts/{attempt_id}/submit": {
      "post": {
        "tags": ["Assessments"],
        "summary": "Submit attempt answers and receive grade",
        "description": "Grade is encrypted at rest. Passing threshold: 70. Letter grade computed automatically. Highest grade is the final grade.",
        "operationId": "submitAttempt",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "attempt_id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/SubmitAttemptRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Grade recorded"},
          "400": {"description": "Already completed or unauthorized"}
        }
      }
    },
    "/api/v1/certifications": {
      "get": {
        "tags": ["Certifications"],
        "summary": "List certifications",
        "operationId": "listCertifications",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "user_id", "in": "query", "schema": {"type": "string", "format": "uuid"}, "description": "Filter by user (defaults to current user)"}],
        "responses": {
          "200": {"description": "Certification list"}
        }
      },
      "post": {
        "tags": ["Certifications"],
        "summary": "Issue certification (admin/instructor)",
        "description": "Certifications expire after 365 days. Duplicate active certs for same user+course are idempotent.",
        "operationId": "issueCertification",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/IssueCertificationRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Certification issued"}
        }
      }
    },
    "/api/v1/reader-artifacts": {
      "get": {
        "tags": ["Reader Artifacts"],
        "summary": "List reader artifacts (bookmarks, highlights, annotations)",
        "operationId": "listReaderArtifacts",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "content_id", "in": "query", "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Artifact list"}
        }
      },
      "post": {
        "tags": ["Reader Artifacts"],
        "summary": "Create reader artifact",
        "description": "Creates immutable history entry for audit trail.",
        "operationId": "createReaderArtifact",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/ReaderArtifact"}
            }
          }
        },
        "responses": {
          "201": {"description": "Artifact created"}
        }
      }
    },
    "/api/v1/orders": {
      "get": {
        "tags": ["Orders"],
        "summary": "List orders",
        "operationId": "listOrders",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"},
          {"name": "status", "in": "query", "schema": {"type": "string", "enum": ["CREATED", "AVAILABLE", "ACCEPTED", "IN_PROGRESS", "COMPLETED", "EXPIRED", "CANCELLED"]}}
        ],
        "responses": {
          "200": {"description": "Paginated order list"}
        }
      },
      "post": {
        "tags": ["Orders"],
        "summary": "Create service order (admin/dispatcher)",
        "description": "Address is encrypted at rest. State machine starts at CREATED.",
        "operationId": "createOrder",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CreateOrderRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Order created"},
          "403": {"$ref": "#/components/responses/Forbidden"}
        }
      }
    },
    "/api/v1/orders/{id}": {
      "get": {
        "tags": ["Orders"],
        "summary": "Get order by ID",
        "operationId": "getOrder",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Order details"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/orders/{id}/status": {
      "patch": {
        "tags": ["Orders"],
        "summary": "Transition order status (admin/dispatcher)",
        "description": "Valid transitions: CREATED→AVAILABLE, AVAILABLE→ACCEPTED/EXPIRED, ACCEPTED→IN_PROGRESS/CANCELLED, IN_PROGRESS→COMPLETED.",
        "operationId": "transitionOrder",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/TransitionRequest"}
            }
          }
        },
        "responses": {
          "200": {"description": "Status updated"},
          "422": {"description": "Invalid state transition"}
        }
      }
    },
    "/api/v1/orders/{id}/accept": {
      "post": {
        "tags": ["Dispatch"],
        "summary": "Accept an order (single winner enforced)",
        "description": "First valid request wins via DB-level SELECT FOR UPDATE locking. Requires idempotency_key. Checks: order must be AVAILABLE, agent must be qualified (courses + grade ≥ 70 + valid certification), workload cap (max 8 open tasks). Losers receive 409 Conflict.",
        "operationId": "acceptOrder",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AcceptOrderRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Order accepted (winner)"},
          "403": {"description": "Agent not qualified"},
          "409": {"$ref": "#/components/responses/Conflict"},
          "422": {"description": "Workload cap exceeded"}
        }
      }
    },
    "/api/v1/orders/{id}/recommendations": {
      "get": {
        "tags": ["Dispatch"],
        "summary": "Get ranked agent recommendations (admin/dispatcher)",
        "description": "Ranking: 50% normalized_distance + 30% reputation_score + 20% workload_penalty. Distance sources: DistanceMatrix → ZIP+4 centroids (haversine) → 50km fallback. Agents at max workload (8) are excluded.",
        "operationId": "recommendAgents",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {
            "description": "Ranked agent list",
            "content": {
              "application/json": {
                "schema": {
                  "allOf": [{"$ref": "#/components/schemas/APIResponse"}],
                  "properties": {
                    "data": {
                      "type": "array",
                      "items": {"$ref": "#/components/schemas/Recommendation"}
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/v1/orders/{id}/payments": {
      "get": {
        "tags": ["Payments"],
        "summary": "List payments for an order",
        "operationId": "listPaymentsByOrder",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Payment list"}
        }
      }
    },
    "/api/v1/orders/{id}/ledger": {
      "get": {
        "tags": ["Ledger"],
        "summary": "List ledger entries for an order",
        "operationId": "listLedgerByOrder",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Ledger entries"}
        }
      }
    },
    "/api/v1/dispatch/expire-stale": {
      "post": {
        "tags": ["Dispatch"],
        "summary": "Expire stale orders and cancel unstarted (admin)",
        "description": "Expires AVAILABLE orders older than 15 minutes. Cancels ACCEPTED orders not started within 2 hours.",
        "operationId": "expireStaleOrders",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {
            "description": "Expiration results",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "expired": {"type": "integer"},
                    "cancelled": {"type": "integer"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/v1/service-zones": {
      "get": {
        "tags": ["Service Zones"],
        "summary": "List service zones",
        "operationId": "listServiceZones",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {"description": "Service zone list"}
        }
      },
      "post": {
        "tags": ["Service Zones"],
        "summary": "Create service zone (admin)",
        "operationId": "createServiceZone",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/ServiceZone"}
            }
          }
        },
        "responses": {
          "201": {"description": "Zone created"}
        }
      }
    },
    "/api/v1/agent-profiles": {
      "post": {
        "tags": ["Agent Profiles"],
        "summary": "Create agent profile (admin)",
        "operationId": "createAgentProfile",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AgentProfile"}
            }
          }
        },
        "responses": {
          "201": {"description": "Profile created"}
        }
      }
    },
    "/api/v1/agent-profiles/{user_id}": {
      "get": {
        "tags": ["Agent Profiles"],
        "summary": "Get agent profile by user ID",
        "operationId": "getAgentProfile",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "user_id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Agent profile"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/invoices": {
      "get": {
        "tags": ["Invoices"],
        "summary": "List invoices",
        "operationId": "listInvoices",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"}
        ],
        "responses": {
          "200": {"description": "Paginated invoice list"}
        }
      },
      "post": {
        "tags": ["Invoices"],
        "summary": "Create invoice (admin/finance)",
        "description": "Billing address encrypted at rest. Tax computed from subtotal × tax_rate.",
        "operationId": "createInvoice",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CreateInvoiceRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Invoice created"}
        }
      }
    },
    "/api/v1/invoices/{id}": {
      "get": {
        "tags": ["Invoices"],
        "summary": "Get invoice by ID",
        "operationId": "getInvoice",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Invoice details"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/invoices/{id}/issue": {
      "post": {
        "tags": ["Invoices"],
        "summary": "Issue a draft invoice (admin/finance)",
        "description": "Transitions DRAFT→ISSUED. Sets due date to 30 days from now.",
        "operationId": "issueInvoice",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Invoice issued"},
          "400": {"description": "Only draft invoices can be issued"}
        }
      }
    },
    "/api/v1/invoices/{id}/payments": {
      "get": {
        "tags": ["Payments"],
        "summary": "List payments for an invoice",
        "operationId": "listPaymentsByInvoice",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Payment list"}
        }
      }
    },
    "/api/v1/payments": {
      "post": {
        "tags": ["Payments"],
        "summary": "Record a payment (admin/finance)",
        "description": "Append-only: no updates to existing records. Duplicate prevention: unique(order_id + amount + method within ±5 minutes). Payment reference encrypted at rest. Auto-reconciles invoice status (PARTIAL/PAID).",
        "operationId": "recordPayment",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CreatePaymentRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Payment recorded"},
          "409": {"$ref": "#/components/responses/Conflict"}
        }
      }
    },
    "/api/v1/payments/{id}": {
      "get": {
        "tags": ["Payments"],
        "summary": "Get payment by ID",
        "operationId": "getPayment",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Payment details"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/refunds": {
      "post": {
        "tags": ["Refunds"],
        "summary": "Process refund as reversal entry (admin/finance)",
        "description": "Creates a debit ledger entry linked to the original payment via LedgerLinks. Does not modify original payment record (append-only).",
        "operationId": "processRefund",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CreateRefundRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Refund processed"},
          "400": {"description": "Refund exceeds original payment or payment not found"}
        }
      }
    },
    "/api/v1/ledger": {
      "get": {
        "tags": ["Ledger"],
        "summary": "List all ledger entries (admin/finance)",
        "operationId": "listLedgerEntries",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"}
        ],
        "responses": {
          "200": {"description": "Paginated ledger entries"}
        }
      }
    },
    "/api/v1/audit-logs": {
      "get": {
        "tags": ["Audit"],
        "summary": "List audit logs (admin)",
        "description": "Each entry includes actor, action, entity, before/after state, and tamper-evident hash chain. Retained 7 years.",
        "operationId": "listAuditLogs",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"},
          {"name": "entity_type", "in": "query", "schema": {"type": "string"}, "description": "Filter by entity type (user, order, payment, etc.)"}
        ],
        "responses": {
          "200": {"description": "Paginated audit logs"}
        }
      }
    },
    "/api/v1/audit-logs/verify": {
      "post": {
        "tags": ["Audit"],
        "summary": "Verify audit log hash chain integrity (admin)",
        "description": "Validates SHA-256 hash chain: hash(previous_hash + current_record) for every entry.",
        "operationId": "verifyAuditChain",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {
            "description": "Chain verification result",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "data": {
                      "type": "object",
                      "properties": {
                        "valid": {"type": "boolean"}
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/v1/config-changes": {
      "get": {
        "tags": ["Config"],
        "summary": "List configuration changes (admin)",
        "operationId": "listConfigChanges",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"}
        ],
        "responses": {
          "200": {"description": "Paginated config changes"}
        }
      }
    },
    "/api/v1/reports": {
      "get": {
        "tags": ["Reports"],
        "summary": "List generated reports (admin)",
        "operationId": "listReports",
        "security": [{"BearerAuth": []}],
        "parameters": [
          {"$ref": "#/components/parameters/PageParam"},
          {"$ref": "#/components/parameters/PerPageParam"}
        ],
        "responses": {
          "200": {"description": "Report list"}
        }
      },
      "post": {
        "tags": ["Reports"],
        "summary": "Generate KPI report (admin)",
        "description": "Exports to CSV in secured local folder with SHA-256 checksum file. KPIs: fulfillment timeliness, exception rate, order volume.",
        "operationId": "generateReport",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/GenerateReportRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Report generated"}
        }
      }
    },
    "/api/v1/reports/{id}": {
      "get": {
        "tags": ["Reports"],
        "summary": "Get report by ID (admin)",
        "operationId": "getReport",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Report details"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/webhooks": {
      "get": {
        "tags": ["Webhooks"],
        "summary": "List webhook subscriptions (admin)",
        "operationId": "listWebhookSubscriptions",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {"description": "Subscription list"}
        }
      },
      "post": {
        "tags": ["Webhooks"],
        "summary": "Create webhook subscription (admin)",
        "description": "LAN-only delivery. HMAC-SHA256 signed with anti-replay nonce. Retry: exponential backoff (max 5). Dead-letter storage for permanent failures. Events: order.created, order.accepted, grade.recorded, etc.",
        "operationId": "createWebhookSubscription",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CreateWebhookRequest"}
            }
          }
        },
        "responses": {
          "201": {"description": "Subscription created"}
        }
      }
    },
    "/api/v1/webhooks/{id}": {
      "get": {
        "tags": ["Webhooks"],
        "summary": "Get webhook subscription by ID (admin)",
        "operationId": "getWebhookSubscription",
        "security": [{"BearerAuth": []}],
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}}],
        "responses": {
          "200": {"description": "Subscription details"},
          "404": {"$ref": "#/components/responses/NotFound"}
        }
      }
    },
    "/api/v1/webhooks/dead-letters": {
      "get": {
        "tags": ["Webhooks"],
        "summary": "List dead-letter webhook deliveries (admin)",
        "operationId": "listDeadLetters",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {"description": "Dead-letter delivery list"}
        }
      }
    },
    "/api/v1/quotas": {
      "get": {
        "tags": ["Quotas"],
        "summary": "Get tenant quota override (admin)",
        "operationId": "getQuotaOverride",
        "security": [{"BearerAuth": []}],
        "responses": {
          "200": {"description": "Quota override"},
          "404": {"description": "No override set (defaults apply)"}
        }
      },
      "put": {
        "tags": ["Quotas"],
        "summary": "Set tenant quota override (admin)",
        "description": "Defaults: 600 RPM, burst 120, 10000 webhook deliveries/day. Override is audit-logged.",
        "operationId": "setQuotaOverride",
        "security": [{"BearerAuth": []}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/QuotaOverride"}
            }
          }
        },
        "responses": {
          "200": {"description": "Quota override set"}
        }
      }
    }
  }
}`
