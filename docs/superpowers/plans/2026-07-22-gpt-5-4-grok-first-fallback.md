# GPT-5.4 Grok-First Fallback Implementation Plan

> **For AI agents:** Required sub-skill: use `executing-plans` to implement this plan. Track progress with the checkboxes below.

**Goal:** Route exact `gpt-5.4` requests through the API key's Grok group first, then switch once to its bound OpenAI group only after Grok is conclusively unavailable and before client-visible output.

**Architecture:** Add a request-local route state shared by Responses, Chat Completions, Messages compatibility, and Responses WebSocket. The state owns the selected API key copy, group subscription, platform, per-route exclusion/retry state, and the one-way Grok-to-OpenAI transition. Add narrow service methods for sticky-route detection and route-switch billing so a restored OpenAI session bypasses Grok while a mid-request switch does not count the user-global RPM twice.

**Tech Stack:** Go, Gin, coder/websocket, PostgreSQL-backed service repositories, Redis scheduler/billing caches, Wire, existing Go test doubles.

---

## Files

- Create `backend/internal/handler/openai_gpt54_grok_fallback.go`: request-local route state, fallback eligibility, bound-group resolution, subscription loading, billing switch, Gin request-context update, and route diagnostics.
- Create `backend/internal/handler/openai_gpt54_grok_fallback_test.go`: focused state-machine, billing, sticky, authorization, and no-output-after-switch tests.
- Modify `backend/internal/service/openai_gateway_scheduling.go`: expose a read-only schedulable sticky-session check for one group.
- Modify `backend/internal/service/openai_ws_forwarder_support.go`: expose a combined session/`previous_response_id` route-continuity check.
- Create `backend/internal/service/openai_gateway_route_sticky_test.go`: service tests for valid, stale, cross-group, and missing sticky bindings.
- Modify `backend/internal/service/billing_cache_service.go`: split group and user RPM checks and add route-switch eligibility that does not increment user-global RPM twice.
- Modify `backend/internal/service/billing_cache_service_rpm_test.go`: verify primary checks count group+user once and route-switch checks count only the destination group.
- Modify `backend/internal/handler/openai_gateway_handler.go`: add subscription dependency and integrate the route state into Responses, Messages, and Responses WebSocket.
- Modify `backend/internal/handler/openai_chat_completions.go`: integrate the same route state into Chat Completions.
- Modify `backend/internal/handler/wire.go`: inject `SubscriptionService` into the OpenAI gateway handler without changing direct test constructors.
- Modify `backend/cmd/server/wire_gen.go`: regenerate Wire output after the provider signature changes.
- Create `backend/internal/handler/openai_gpt54_grok_fallback_integration_test.go`: endpoint-level routing tests for the four ingress paths.
- Modify `PRIVACYFILTER_WORKFLOW.md`: record this behavior as a required downstream customization and add upgrade acceptance checks.
- Create deployment/build records under the repository's existing `docs/superpowers` conventions after verification.

### Task 1: Route State and Authorization

- [x] **Step 1: Write failing route-state tests**

Add table tests proving that only normalized exact `gpt-5.4` with a Grok primary is eligible; direct `grok-*`, other `gpt-*`, nil groups, and an already selected OpenAI route are unchanged. Test that fallback resolution uses `service.ResolveAPIKeyRequestPlatform(source, service.PlatformOpenAI)`, rejects unbound/ambiguous groups, retains all original group bindings, and maintains independent failed-account/retry maps.

- [x] **Step 2: Run the focused test and confirm RED**

Run: `go test ./internal/handler -run 'TestGPT54GrokFirstRoute' -count=1`

Expected: FAIL because the route state and transition helpers do not exist.

- [x] **Step 3: Implement the minimum request-local state**

Implement an unexported `openAIGPT54Route` with source/current API keys, current subscription/platform, separate Grok/OpenAI attempt state, switched flag, output-started guard, primary error, and fallback reason. Implement a one-way transition that resolves only an active OpenAI group bound to the same API key and never mutates the authenticated key.

- [x] **Step 4: Run the focused test and confirm GREEN**

Run: `go test ./internal/handler -run 'TestGPT54GrokFirstRoute' -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```powershell
git add backend/internal/handler/openai_gpt54_grok_fallback.go backend/internal/handler/openai_gpt54_grok_fallback_test.go
git commit -m "feat: add gpt-5.4 grok-first route state"
```

### Task 2: Sticky Continuity and Route-Switch Billing

- [x] **Step 1: Write failing service tests**

Add tests proving that an eligible OpenAI sticky session or valid OpenAI `previous_response_id` binding selects the fallback route before Grok; stale, disabled, wrong-group, wrong-model, and missing bindings do not. Extend RPM tests with a counting cache: normal eligibility increments `(user, Grok group)` and user RPM once; route-switch eligibility increments only `(user, OpenAI group)` and never increments user RPM again.

- [x] **Step 2: Run the focused tests and confirm RED**

Run: `go test ./internal/service -run 'TestOpenAIGatewayHasRouteContinuity|TestBillingCacheRouteSwitch' -count=1`

Expected: FAIL because the continuity and route-switch billing APIs do not exist.

- [x] **Step 3: Implement sticky detection and billing separation**

Expose a service method that checks a group-scoped session sticky and `previous_response_id` binding using the existing eligibility, capability, quota-pause, and group-membership rules. Refactor `checkRPM` into group and user portions while preserving `CheckBillingEligibility`; add `CheckBillingEligibilityForRouteSwitch` that repeats balance/subscription/platform/API-key eligibility and destination-group RPM checks but skips the user-global RPM increment.

- [x] **Step 4: Run service tests and confirm GREEN**

Run: `go test ./internal/service -run 'TestOpenAIGatewayHasRouteContinuity|TestBillingCacheRouteSwitch|TestBillingCache.*RPM' -count=1`

Expected: PASS with exact counter assertions.

- [x] **Step 5: Commit**

```powershell
git add backend/internal/service/openai_gateway_scheduling.go backend/internal/service/openai_ws_forwarder_support.go backend/internal/service/openai_gateway_route_sticky_test.go backend/internal/service/billing_cache_service.go backend/internal/service/billing_cache_service_rpm_test.go
git commit -m "feat: preserve openai fallback route continuity"
```

### Task 3: HTTP Responses and Chat Completions

- [x] **Step 1: Write failing endpoint tests**

Build handler tests with one bound Grok group and one bound OpenAI group. Cover: healthy Grok wins; Grok `WaitPlan` never switches; no Grok candidates switches to OpenAI; all failover-eligible Grok attempts switch once; non-retryable 400, cancellation, billing rejection, and any written response do not switch; OpenAI unbound/ambiguous returns routing error; successful fallback records OpenAI group/subscription/account and binds OpenAI sticky state; non-`gpt-5.4` behavior is unchanged.

- [x] **Step 2: Run tests and confirm RED**

Run: `go test ./internal/handler -run 'TestGPT54GrokFirst(Responses|ChatCompletions)' -count=1`

Expected: FAIL because both handlers currently terminate after Grok exhaustion.

- [x] **Step 3: Integrate the shared route into both loops**

Generate session identity before the first route billing check, restore a valid OpenAI fallback sticky route before scheduling, and recompute group channel mapping after a transition. On `ErrNoAvailableAccounts`, nil selection, or exhausted failover-eligible Grok attempts, transition only while the request is replayable and output has not started. Use current-route API key, group, platform, subscription, failed IDs, concurrency binding, cyber record, quota platform, usage input, and logs on every attempt.

- [x] **Step 4: Run tests and confirm GREEN**

Run: `go test ./internal/handler -run 'TestGPT54GrokFirst(Responses|ChatCompletions)' -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```powershell
git add backend/internal/handler/openai_gateway_handler.go backend/internal/handler/openai_chat_completions.go backend/internal/handler/openai_gpt54_grok_fallback_integration_test.go
git commit -m "feat: fallback gpt-5.4 http requests after grok exhaustion"
```

### Task 4: Messages Compatibility and Responses WebSocket

- [x] **Step 1: Add failing endpoint tests**

Add Messages tests for mapped `gpt-5.4 -> grok-4.5`, Grok wait/no-account/failover/non-retryable/output-started cases, and OpenAI group usage after fallback. Add WebSocket tests for Grok-first selection, busy Grok rejection without fallback, pre-frame 429/no-account transition, no transition after any client-visible frame, route-specific transport/capability, and restored OpenAI session continuity.

- [x] **Step 2: Run tests and confirm RED**

Run: `go test ./internal/handler -run 'TestGPT54GrokFirst(Messages|ResponsesWebSocket)' -count=1`

Expected: FAIL because Messages and WebSocket still terminate within the Grok route.

- [x] **Step 3: Integrate route state into Messages and WebSocket**

Use the same transition API and current-route state. Messages must recompute its dispatch and channel mappings after switching. WebSocket may switch only before a downstream frame; Grok remains HTTP-SSE transport while fallback OpenAI recomputes WS transport and capability. Hook closures must capture current-route API key/subscription so every turn's billing and usage stay in the selected OpenAI group after fallback.

- [x] **Step 4: Run tests and confirm GREEN**

Run: `go test ./internal/handler -run 'TestGPT54GrokFirst(Messages|ResponsesWebSocket)' -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```powershell
git add backend/internal/handler/openai_gateway_handler.go backend/internal/handler/openai_gpt54_grok_fallback_integration_test.go
git commit -m "feat: extend gpt-5.4 grok fallback to messages and websocket"
```

### Task 5: Dependency Wiring and Downstream Documentation

- [x] **Step 1: Add a failing provider contract test**

Update the existing Wire generation test to require `SubscriptionService` in `ProvideOpenAIGatewayHandler` and verify the handler receives it. Add a workflow assertion or documentation check covering the new customization.

- [x] **Step 2: Run tests and confirm RED**

Run: `go test ./cmd/server ./internal/handler -run 'TestWireGen|TestGPT54GrokFirstDependency' -count=1`

Expected: FAIL until the provider and generated graph are updated.

- [x] **Step 3: Wire the subscription dependency and document the customization**

Add the narrow subscription resolver field, inject the existing `SubscriptionService` in `ProvideOpenAIGatewayHandler`, run `wire`/the repository generator to update `wire_gen.go`, and append the exact fallback rules plus upgrade acceptance checklist to `PRIVACYFILTER_WORKFLOW.md`.

- [x] **Step 4: Run focused packages and confirm GREEN**

Run: `go test ./cmd/server ./internal/handler ./internal/service -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```powershell
git add backend/internal/handler/wire.go backend/cmd/server/wire_gen.go backend/cmd/server/wire_gen_test.go PRIVACYFILTER_WORKFLOW.md
git commit -m "docs: retain gpt-5.4 grok-first fallback customization"
```

### Task 6: Full Verification, Build, and VPS Deployment

- [x] **Step 1: Format and run backend verification**

Run: `gofmt -w <all changed Go files>`

Run: `go test ./internal/handler ./internal/service ./internal/server/middleware ./cmd/server -count=1`

Run: `go test ./... -count=1`

Expected: all commands exit 0.

- [x] **Step 2: Run frontend/customization regression checks**

Run the repository's documented frontend test and build commands, including model defaults, multi-group keys, Grok defaults, Forbidden cleanup, and `openai_passthrough` coverage.

Expected: all commands exit 0 and no required v161 customization regresses.

- [x] **Step 3: Build production artifacts and record provenance**

Use the v161 `PRIVACYFILTER_WORKFLOW.md` build procedure. Record source HEAD, dirty state, toolchain versions, artifact hashes, and included downstream features. Never include secrets, database dumps, or production tokens.

- [x] **Step 4: Deploy to the US VPS with rollback protection**

Using `us_ssh`, capture the running version and health, back up the current binary/image and service configuration, upload the verified artifact, restart only the Sub2API service, and wait for readiness. If readiness fails, restore the backup immediately.

- [x] **Step 5: Run production smoke tests**

With the existing CC Switch API key, verify a normal new `gpt-5.4` session selects Grok and maps upstream to `grok-4.5`. Temporarily make all Grok accounts unschedulable without deleting or editing credentials, verify a new session selects the key-bound OpenAI group, then restore the original Grok states. Confirm a successful fallback session remains on OpenAI, billing/usage/log group IDs match the actual route, and other models are unchanged.

- [x] **Step 6: Commit build and deployment records**

```powershell
git add -f docs/superpowers/<build-and-deployment-records>
git commit -m "docs: record gpt-5.4 grok-first deployment"
```
