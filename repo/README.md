# DispatchLearn Operations Settlement System

A production-grade, offline-first field service + LMS + settlement platform built with Go (Gin), GORM, and MySQL.

## Quick Start

```bash
docker-compose up --build
```

The API will be available at `http://localhost:8080`.

## Default Credentials

| User   | Username | Password  | Role  |
|--------|----------|-----------|-------|
| Admin  | admin    | admin123  | admin |
| Agent  | agent1   | admin123  | agent |

**Tenant ID:** `00000000-0000-0000-0000-000000000001`

## API Endpoints

### Health
- `GET /health`

### Auth
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`

### Users & Sessions
- `GET /api/v1/me`
- `GET /api/v1/users`
- `GET /api/v1/users/:id`
- `POST /api/v1/users/:id/roles`
- `GET /api/v1/sessions`
- `DELETE /api/v1/sessions/:session_id`

### LMS
- `POST/GET /api/v1/courses`
- `GET /api/v1/courses/:id`
- `POST /api/v1/courses/:id/content`
- `POST /api/v1/courses/:id/assessments`
- `POST /api/v1/assessments/:id/attempts`
- `POST /api/v1/attempts/:id/submit`
- `POST/GET /api/v1/certifications`
- `POST/GET /api/v1/reader-artifacts`

### Dispatch
- `POST/GET /api/v1/orders`
- `GET /api/v1/orders/:id`
- `PATCH /api/v1/orders/:id/status`
- `POST /api/v1/orders/:id/accept`
- `GET /api/v1/orders/:id/recommendations`
- `POST/GET /api/v1/service-zones`
- `POST /api/v1/agent-profiles`
- `GET /api/v1/agent-profiles/:user_id`

### Finance
- `POST/GET /api/v1/invoices`
- `POST /api/v1/invoices/:id/issue`
- `POST /api/v1/payments`
- `GET /api/v1/payments/:id`
- `POST /api/v1/refunds`
- `GET /api/v1/ledger`

### System
- `GET /api/v1/audit-logs`
- `POST /api/v1/audit-logs/verify`
- `GET /api/v1/config-changes`
- `POST/GET /api/v1/reports`
- `POST/GET /api/v1/webhooks`
- `GET/PUT /api/v1/quotas`

## Running Tests

```bash
./run_tests.sh
```

This builds Docker images, starts MySQL + app, runs unit and API tests, then outputs a summary.

## Architecture

```
cmd/server/          - Application entry point
config/              - Centralized configuration
logging/             - Structured logging
internal/
  domain/            - Domain models (entities)
  repository/        - Data access (GORM)
  usecase/           - Business logic
  handler/           - HTTP handlers (Gin)
  middleware/        - Auth, RBAC, rate limiting, logging
  crypto/            - AES-256 encryption
  audit/             - Tamper-evident audit logging
tests/
  unit/              - Unit tests
  api/               - API integration tests
scripts/             - Backup/restore scripts
```

## Environment Variables

All defined in `docker-compose.yml`:

| Variable | Default | Description |
|----------|---------|-------------|
| SERVER_PORT | 8080 | HTTP port |
| GIN_MODE | release | Gin mode |
| DB_HOST | mysql | MySQL host |
| DB_PORT | 3306 | MySQL port |
| DB_USER | dispatchlearn | DB user |
| DB_PASSWORD | dispatchlearn_secret | DB password |
| DB_NAME | dispatchlearn | Database name |
| JWT_SECRET | (set) | JWT signing secret |
| JWT_MAX_SESSIONS | 10 | Max sessions per user |
| ENCRYPTION_MASTER_KEY | (set) | AES-256 master key |
| ENABLE_TLS | false | Enable TLS |
| QUOTA_RPM | 600 | Rate limit per minute |
| QUOTA_BURST | 120 | Burst limit |
| QUOTA_WEBHOOK_DAILY | 10000 | Webhook daily cap |

## Backup & Recovery

```bash
# Backup (inside container)
docker-compose exec app bash /app/scripts/backup.sh

# Restore
docker-compose exec app bash /app/scripts/restore.sh /app/backups/<file>.sql.gz
```

RPO: 15 minutes | RTO: 4 hours
