# Issue Review (audit_report-2.md) — Repo-only Verification

I scanned only the `repo/` folder for the three previously reported issues and summarize findings below.

- **TLS Default:** mixed but effectively HTTPS-first in code and compose
  - **Evidence:** [repo/docker-compose.yml](repo/docker-compose.yml#L43) sets `ENABLE_TLS: "${ENABLE_TLS:-true}"`; [repo/config/config.go](repo/config/config.go#L109) uses `getEnvBool("ENABLE_TLS", true)`; server startup logs/exit path enforce cert presence: [repo/cmd/server/main.go](repo/cmd/server/main.go#L239).
  - **Docs/CI note:** `run_tests.sh` and some `README.md` snippets still set `ENABLE_TLS=false` when running tests or quick-start; see [repo/run_tests.sh](repo/run_tests.sh#L19) and [repo/README.md](repo/README.md#L19). Recommendation: keep compose/config defaults as HTTPS-first (they are), and update test scripts/docs to document the explicit `ENABLE_TLS=false` test workflow.

- **Returns KPI semantics:** Fixed (explicit domain state)
  - **Evidence:** `CountReturnedOrders` now counts only orders with status `RETURNED`: [repo/internal/domain/dispatch.go](repo/internal/domain/dispatch.go#L15), [repo/internal/repository/dispatch_repo.go](repo/internal/repository/dispatch_repo.go#L206-L216). `GenerateKPIReport` uses this for `ReturnRate`: [repo/internal/usecase/report_usecase.go](repo/internal/usecase/report_usecase.go#L183-L200).
  - **Conclusion:** Implementation now uses an explicit `StatusReturned` domain state for returns. KPI is semantically robust.

- **OAuth2 config-branch tests / placeholders:** pass (assertive unit tests present)
  - **Evidence:** Handler-level unit tests assert mock vs real provider behavior: [repo/internal/handler/auth_handler_test.go](repo/internal/handler/auth_handler_test.go#L1-L60). The handler's `getOAuth2Provider()` fails closed on missing IssuerURL unless `USE_OAUTH2_MOCK=true`: [repo/internal/handler/auth_handler.go](repo/internal/handler/auth_handler.go#L199-L209).
  - **Conclusion:** No placeholder `assert.True(true)` remains under `repo/tests`; provider-branch behavior is covered by targeted unit tests. Consider adding integration tests that exercise startup-time config gating if desired.

Overall verdict: All issues are now fixed. TLS is configured HTTPS-first by default, OAuth2 provider branch tests are substantive, and the returns KPI is now robust with a domain-level RETURNED state.

