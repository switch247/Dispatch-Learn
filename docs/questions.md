Below are up to 15 concise clarification questions derived from the original prompt, each with context, my understanding, and a suggested solution.

1) Question: Tenant isolation strategy?
- Context: Backup/restore, migrations, query enforcement, and audit boundaries depend on this.
- My understanding: The prompt requires `tenant_id` on every table but doesn't specify DB-per-tenant vs single DB.
- Suggested solution: Use a single MySQL instance with a mandatory `tenant_id` column enforced by DB constraints and application-level RBAC. Document rationale in `ASSUMPTIONS.md`.

2) Question: Is a global/system admin allowed cross-tenant access?
- Context: Needed for seed/demo data, emergency recovery, and migrations.
- My understanding: Prompt implies multi-tenant but doesn't define global admin.
- Suggested solution: Define a `system` role with explicit cross-tenant privileges; require MFA for that account and audit every cross-tenant action.

3) Question: Can users hold multiple roles concurrently and switch active role?
- Context: Affects permission checks, audit trails, and UI behavior.
- My understanding: Roles/Permissions exist but multiplicity not defined.
- Suggested solution: Implement many-to-many `user_roles`; allow role-switching but record `previous_role`/`actor_id` in AuditLogs.

4) Question: Tenant identifier type and uniqueness constraints?
- Context: Impacts indexing, FK relationships, and URL routing.
- My understanding: Tenant will be required on all domain tables, but format unknown.
- Suggested solution: Use UUID v4 `tenant_id` (binary(16) indexed) and enforce via FK-like constraints and middleware.

5) Question: Encryption key management specifics (KMS vs local keys)?
- Context: AES-256 + rotation every 180 days required; needs key storage & rotation process.
- My understanding: No external KMS allowed at runtime.
- Suggested solution: Implement local key store files encrypted by a master passphrase stored in secure operator config; rotate keys via CLI migration job that re-encrypts data and appends key metadata to `ConfigChanges`.

6) Question: Webhook anti-replay nonce lifetime and storage?
- Context: Preventing replay while supporting retries and dead-lettering.
- My understanding: Nonce required but retention/garbage unclear.
- Suggested solution: Store nonces for 7 days with associated `delivery_id`; reject duplicates; TTL cleanup job.

7) Question: Webhook filtering semantics (date-range behavior)?
- Context: Subscriptions filter by tenant, event type, and effective date range—how to interpret ranges for historical replay?
- My understanding: Filters should apply both to live events and replay.
- Suggested solution: Treat ranges as inclusive; for replay, deliver events whose `event_time` falls in the subscription's configured range.

8) Question: JWT + refresh token rotation and session caps enforcement?
- Context: Tokens expire after 30 minutes, rotating refresh tokens, max 10 active sessions per user.
- My understanding: Rotation semantics unspecified (stateless vs tracked).
- Suggested solution: Track refresh tokens in `UserSessions` with issued_at, revoked_at; on refresh, rotate (invalidate previous), and enforce max 10 active sessions by rejecting new logins or removing oldest session per policy.

9) Question: Password policy and lockout thresholds?
- Context: Users table includes `failed_attempt_count`; locking policy affects security.
- My understanding: Not specified.
- Suggested solution: Default lockout after 5 failed attempts for 30 minutes; admins may adjust via ConfigChanges; log all lock/unlock events.

10) Question: Distance data sources and fallbacks (ZIP+4 coverage)?
- Context: Distance ranking must be offline-safe; source availability varies by region.
- My understanding: Prompt allows ZIP+4 centroids, precomputed matrices, or manual tables.
- Suggested solution: Accept multiple sources with precedence: `DistanceMatrix` → `Zip4Centroids` → `DistanceOverrides`. If ZIP+4 missing, fall back to zone centroid.

11) Question: How to compute and store ZIP+4 centroids and updates?
- Context: Centroid dataset size and update process affect storage and offline use.
- My understanding: Dataset must be provided and maintained locally.
- Suggested solution: Store centroids in `Zip4Centroids` table with `source_version`; provide CLI import and validation steps; record imports in `ConfigChanges`.

12) Question: Workload cap enforcement timing and open-task definition?
- Context: Max 8 open tasks per agent—how to count "open"?
- My understanding: Open likely includes CREATED/AVAILABLE/ACCEPTED/IN_PROGRESS except COMPLETED/CANCELLED.
- Suggested solution: Define open states as {AVAILABLE, ACCEPTED, IN_PROGRESS}; enforce in recommendation service and on acceptance with DB-level check.

13) Question: Acceptance idempotency and winner selection ties?
- Context: Need single winner, idempotency_key uniqueness, and locking.
- My understanding: First valid request wins; losers get 409.
- Suggested solution: Require client `Idempotency-Key`; implement DB transaction with `SELECT ... FOR UPDATE` on Orders row, check existing acceptance, insert `DispatchAcceptance` with `idempotency_key` unique constraint.

14) Question: Payments append-only and duplicate detection window?
- Context: Duplicate prevention: unique(order_id + amount + method within ±5 minutes).
- My understanding: Time window ambiguous for matching.
- Suggested solution: On payment insert, search for existing payment with same `order_id`, `amount`, `method` where `created_at` BETWEEN now() -5min AND now() +5min; if found, reject as duplicate; otherwise insert.

15) Question: Audit log tamper-evident hashing and storage retention?
- Context: Hash(previous_hash + current_record) required and retention 7 years.
- My understanding: Storage medium and verification process unspecified.
- Suggested solution: Implement chained SHA-256 hashes stored in `AuditLogs` with `previous_hash`; provide a CLI `verify-audit` tool to validate chain integrity; rotate archive to secure offline storage with retention metadata.

Would you like me to (A) apply any of the suggested defaults into `ASSUMPTIONS.md`, or (B) adjust specific recommendations before I generate the file?
