# ASSUMPTIONS.md

Documented assumptions made during the implementation of DispatchLearn.

## Tenant Isolation

- **Single MySQL instance** with mandatory `tenant_id` column on all domain tables.
- All queries are scoped by `tenant_id` at the repository layer.
- No cross-tenant joins permitted; violations return 403 Forbidden.
- A `system_admin` role exists for emergency cross-tenant access (all actions audited).

## Authentication & Sessions

- Username/password authentication with bcrypt-hashed passwords.
- JWT access tokens expire after 30 minutes; refresh tokens rotate on use.
- Maximum 10 active sessions per user; oldest session is evoked when cap is exceeded.
- Account lockout after 5 consecutive failed login attempts for 30 minutes.
- Lockout events are logged in the audit trail.

## Encryption

- AES-256-GCM encryption for sensitive fields (grades, payment references, addresses).
- Master key is provided via environment variable `ENCRYPTION_MASTER_KEY` (32 bytes).
- Key rotation every 180 days requires a migration job to re-encrypt data.
- No external KMS used; master key stored in secure operator configuration.

## Qualified Dispatch

- An agent is eligible for assignment only if:
  - All required courses for the order's category are completed.
  - Latest grade >= 70 (passing threshold).
  - Certification (if required) is not expired (365-day validity).
- Override requires `override_reason` and is logged in audit.
- When no courses are required for a category, all agents are considered qualified.

## Dispatch State Machine

- `CREATED → AVAILABLE → ACCEPTED → IN_PROGRESS → COMPLETED`
- Orders auto-expire if not accepted within 15 minutes of becoming AVAILABLE.
- Accepted orders auto-cancel if not started within 2 hours.
- Only one acceptance per order, enforced by `unique(order_id)` constraint and DB-level row locking (`SELECT ... FOR UPDATE`).
- Losers receive 409 Conflict.

## Workload Cap

- Max 8 open tasks per agent (AVAILABLE, ACCEPTED, IN_PROGRESS).
- Agents at capacity are excluded from recommendation lists.

## Reputation Score

- Formula: `50% fulfillment_timeliness + 30% average_grade + 20% completion_rate`
- Minimum 5 completed orders required; default score is 50 until threshold met.
- Updated daily (designed for cron invocation).

## Distance Ranking

- Formula: `50% normalized_distance + 30% reputation_score + 20% workload_penalty`
- Distance sources in order of precedence:
  1. Precomputed `DistanceMatrix` (zone-to-zone)
  2. `Zip4Centroids` (haversine calculation)
  3. Default fallback of 50km
- No external map services used.

## Settlement & Payments

- Payments are append-only; no updates to existing payment records.
- Duplicate prevention: `unique(order_id + amount + method within ±5 minutes)`.
- Refunds are implemented as reversal ledger entries, linked via `LedgerLinks`.
- Invoice reconciliation: auto-transitions to PAID when total payments >= invoice total.

## LMS

- Maximum 3 attempts per assessment (configurable per assessment).
- Highest grade is the final grade.
- Passing threshold: 70.
- Certifications expire after 365 days.
- Reader artifacts (bookmarks, highlights, annotations) have immutable history.

## Audit Logging

- Tamper-evident: each log entry contains `hash(previous_hash + current_record)` using SHA-256.
- Chain verification API available to validate integrity.
- Retention: designed for 7 years (application-level; storage management is operational).
- All state changes on domain entities are logged with before/after state.

## Webhooks

- LAN-only delivery (external services are mocked/stubbed).
- HMAC-SHA256 signatures with anti-replay nonce.
- Exponential backoff retry (max 5 attempts).
- Dead-letter storage for permanently failed deliveries.
- Delivery IDs are unique and idempotent.

## Quotas

- Default: 600 requests/minute, burst 120.
- Webhooks: 10,000 deliveries/day.
- Per-tenant overrides supported.
- Violations are logged.

## Backup & Recovery

- Nightly full backup via mysqldump.
- MySQL binlog retention: 15 minutes (`binlog-expire-logs-seconds=900`).
- RPO: 15 minutes (binlog + nightly full).
- RTO: 4 hours (restore script + validation).
- SHA-256 checksums for all backup files.

## External Services

- All third-party integrations are **mocked/stubbed** as per project requirements.
- Webhook delivery simulates LAN POST operations with logging.
- No real payment gateway integration; payments are recorded locally.

## Date/Time

- All timestamps are stored and processed in UTC.
- Time parsing uses RFC3339 format.

## Content Items

- Maximum file size: 50MB per content item.
- Supported types: epub, pdf, html.
- Checksums stored for integrity verification.
