# Issue Review — Static Verification Results

Summary of verification for issues reported in the previous delivery audit.

- **TLS Default:** Fixed — docker-compose and config default to TLS enabled.
  - **Evidence:** [repo/docker-compose.yml](repo/docker-compose.yml#L43) (ENABLE_TLS default) ; [repo/config/config.go](repo/config/config.go#L109) (getEnvBool default=true).
  - **Conclusion:** Resolved. Deployment quick-start now prefers HTTPS by default.

- **Returns KPI semantics:** Fixed (explicit domain state).
  - **Evidence:** `CountReturnedOrders` now counts only orders with status `RETURNED`: [repo/internal/domain/dispatch.go](repo/internal/domain/dispatch.go#L15), [repo/internal/repository/dispatch_repo.go](repo/internal/repository/dispatch_repo.go#L206-L216).
  - **Conclusion:** Implementation now uses an explicit `StatusReturned` domain state for returns. KPI is semantically robust.

- **OAuth2 config-branch tests / placeholders:** Fixed.
  - **Evidence:** Assertive unit tests for provider selection: [repo/internal/handler/auth_handler_test.go](repo/internal/handler/auth_handler_test.go#L1-L40). Handler enforces fail-closed when misconfigured: [repo/internal/handler/auth_handler.go](repo/internal/handler/auth_handler.go#L199-L209).
  - **Conclusion:** Tests validate mock vs real provider branches; no placeholder `assert.True(true)` remains in `tests/api`.

Overall conclusion: All high-severity items are resolved (TLS, OAuth2 tests, and Returns KPI). The code now uses explicit domain state for returns, and TLS defaults and test/documentation paths are clear and robust.

