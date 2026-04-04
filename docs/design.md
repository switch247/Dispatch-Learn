# Design Notes — DispatchLearn

Purpose & scope
- Backend service providing field dispatch, learning management (LMS), and finance/settlement capabilities. Built for multi-tenant deployments with strong auditability and RBAC.

High-level architecture
- Language & frameworks: Go, Gin (HTTP), GORM (ORM)
- Layers:
  - `cmd/` — application entrypoint and wiring ([repo/cmd/server/main.go](repo/cmd/server/main.go))
  - `config/` — configuration loading and validation
  - `internal/domain/` — domain entities and DTOs
  - `internal/repository/` — data access (GORM)
  - `internal/usecase/` — business logic and orchestration
  - `internal/handler/` — HTTP handlers (Gin)
  - `internal/middleware/` — auth, tenant isolation, RBAC, rate limiting, quotas
  - `internal/worker/` — background jobs (expiry worker, etc.)

Data model highlights
- Multi-tenant root model (`Tenant`) — tenant id required on most models.
- Auth: `User`, `Role`, `Permission`, `UserSession`.
- LMS: `Course`, `ContentItem`, `Assessment`, `AssessmentAttempt`, `Certification`.
- Dispatch: `Order`, `AgentProfile`, `ServiceZone`, `DispatchAcceptance`.
- Finance: `Invoice`, `Payment`, `LedgerEntry`, `LedgerLink`.

Security & operational concerns
- Authentication: OAuth2 supported, JWT for API sessions.
- Encryption: AES-256 master key used via `internal/crypto` for sensitive fields.
- Audit: tamper-evident audit logs via `internal/audit`.
- RBAC: role + permission model, middleware-enforced with `RequireRole` wrappers.
- Tenancy: tenant isolation middleware ensures per-tenant scoping.
- Rate limiting & quotas: DB-backed quota provider with in-memory cache.

Deployment & infra
- Docker-compose provided for local development; production uses TLS and controlled env vars (see `docker-compose.yml`).
- Auto-migration on startup; initial seed data created by `seedData` in the server bootstrap.

Extensibility points
- Add new HTTP routes in `internal/handler/` and register them in `cmd/server/main.go` under `/api/v1`.
- Add persistence logic in `internal/repository/` and wire into usecases.
- Background jobs belong in `internal/worker/` and are started from `cmd/server`.

Testing
- Unit tests in `tests/unit/` and integration/API tests in `tests/api/`.
- Use `./run_tests.sh` to run the test suite and service-dependent integration tests.

Where to start reading code
- Start with the README for quick start: [repo/README.md](repo/README.md)
- See server wiring: [repo/cmd/server/main.go](repo/cmd/server/main.go)
- Domain types: [repo/internal/domain/auth.go](repo/internal/domain/auth.go)
