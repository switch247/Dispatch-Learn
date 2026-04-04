# API Specification — DispatchLearn

Overview
- DispatchLearn is a field-service + LMS + settlement backend written in Go (Gin + GORM). It exposes a versioned HTTP JSON API, supports multi-tenant operation, RBAC, OAuth2/JWT authentication, and background workers for expiry/processing.

Base URL
- Local/dev: `http://localhost:8080`
- API prefix: `/api/v1`

Authentication
- OAuth2 + JWT access tokens. Endpoints that issue/refresh tokens are public under `/api/v1/auth`.
- Typical flow: `POST /api/v1/auth/login` -> return `access_token` + `refresh_token`.

Primary endpoints (overview)
- Health: `GET /health`
- Docs: `GET /docs`, `GET /api/v1/openapi.json`

Auth
- `POST /api/v1/auth/register` — register a user
- `POST /api/v1/auth/login` — login (returns JWT + refresh token)
- `POST /api/v1/auth/refresh` — refresh access token
- `GET /api/v1/auth/oauth2/login` & `POST /api/v1/auth/oauth2/callback` — external OAuth2

Users & Sessions
- `GET /api/v1/me` — current user
- `GET /api/v1/users` — list users (admin)
- `GET /api/v1/sessions` — list sessions
- `DELETE /api/v1/sessions/:session_id` — revoke session

LMS
- `POST/GET /api/v1/courses`
- `POST /api/v1/courses/:id/content`
- `POST /api/v1/assessments/:id/attempts`

Dispatch
- `POST/GET /api/v1/orders`
- `PATCH /api/v1/orders/:id/status`
- `POST /api/v1/orders/:id/accept`

Finance
- `POST/GET /api/v1/invoices`
- `POST /api/v1/payments`

System
- `GET /api/v1/audit-logs`
- `POST /api/v1/webhooks`
- `POST/GET /api/v1/reports`

Request/response format
- All responses use a standard JSON wrapper (`APIResponse`) with `data`, optional `meta`, and `errors` fields. List endpoints include pagination meta.

Errors
- Validation errors return `422` with `VALIDATION_ERROR` code.
- Conflict returns `409` with `CONFLICT` code.

Example — login request

POST /api/v1/auth/login

{
  "username": "admin",
  "password": "admin123",
  "tenant_id": "00000000-0000-0000-0000-000000000001"
}

Response (success)

{
  "data": {
    "access_token": "<jwt>",
    "refresh_token": "<token>",
    "expires_in": 3600,
    "token_type": "bearer"
  }
}

Running & testing
- Quick start: `docker-compose up --build` (app + MySQL). See repository README.
- Run tests locally: `./run_tests.sh`

Useful files
- Project README: [repo/README.md](repo/README.md)
- Server entrypoint: [repo/cmd/server/main.go](repo/cmd/server/main.go)
