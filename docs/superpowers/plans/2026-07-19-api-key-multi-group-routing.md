# API Key Multi-Group Routing And Grok Defaults Implementation Plan

> **For the implementing AI agent:** Required sub-skill: use `executing-plans` in the current session. Track every step with checkboxes and stop at the deployment checkpoint if local verification is not fully green.

**Goal:** Let one API key route requests across Codex, Grok, and Claude groups, while preserving legacy single-group behavior and adding the confirmed Grok account defaults.

**Architecture:** Add an `api_key_groups` join table while retaining `api_keys.group_id` as the default group. Authentication loads all bindings, then a request-scoped resolver selects one group from the requested model before subscription, billing, dispatch, account selection, sticky sessions, and logging. Grok account defaults are applied in both the create UI and backend create service without overwriting explicit caller configuration.

**Tech Stack:** Go 1.24, Gin, Ent, PostgreSQL, Redis-backed auth/scheduler caches, Vue 3, TypeScript, Vitest, pnpm.

---

## File Structure

### Backend data and API key domain

- Create `backend/migrations/185_api_key_groups.sql`: additive join table and legacy binding backfill.
- Create `backend/migrations/api_key_groups_migration_test.go`: migration contract assertions.
- Create `backend/ent/schema/api_key_group.go`: Ent edge schema for the join table.
- Modify `backend/ent/schema/api_key.go`: add binding edge while retaining legacy default group edge.
- Modify `backend/ent/schema/group.go`: add reverse API key binding edge.
- Regenerate `backend/ent/*`: generated entity, query, mutation, create, update, and migration files.
- Modify `backend/internal/service/api_key.go`: add `GroupIDs`, `Groups`, and request-local selection helper.
- Modify `backend/internal/repository/api_key_repo.go`: dual-read and transactional dual-write of default and bound groups.
- Modify `backend/internal/repository/api_key_repo_test.go`: repository unit coverage.
- Modify `backend/internal/repository/migrations_schema_integration_test.go`: live schema coverage.

### Routing and authentication

- Create `backend/internal/service/api_key_group_router.go`: pure model-to-platform/group resolver.
- Create `backend/internal/service/api_key_group_router_test.go`: alias, family, fallback, and error cases.
- Modify `backend/internal/server/middleware/api_key_auth.go`: extract model, resolve request-local group, then resolve subscription.
- Modify `backend/internal/server/middleware/api_key_auth_google.go`: keep Google authentication behavior compatible with multi-group keys.
- Modify `backend/internal/server/middleware/api_key_auth_test.go`: middleware routing and non-mutation coverage.
- Modify `backend/internal/server/routes/gateway.go`: ensure dispatch observes the selected request-local group.
- Modify `backend/internal/server/routes/gateway_test.go`: route dispatch coverage for all three platforms.

### API key handlers and frontend

- Modify `backend/internal/handler/api_key_handler.go`: accept/return `group_ids` and validate default membership.
- Modify `backend/internal/handler/dto/types.go`: add API key `group_ids`/`groups` response fields.
- Modify `backend/internal/handler/dto/mappers.go`: map all bindings.
- Modify `backend/internal/handler/api_key_handler_test.go`: create/update compatibility tests.
- Modify `frontend/src/types/index.ts` or the existing API key type module: expose `group_ids` and `groups`.
- Modify `frontend/src/api/keys.ts`: send `group_ids` plus the default `group_id`.
- Modify `frontend/src/views/user/KeysView.vue`: multi-select bound groups and select one default group.
- Modify `frontend/src/views/user/__tests__/KeysView.spec.ts`: multi-group UI behavior.
- Modify `frontend/src/components/__tests__/ApiKeyCreate.spec.ts`: create payload regression coverage.
- Modify Chinese and English key locale files under `frontend/src/i18n/locales/*`: concise multi-group/default labels and errors.

### Grok account defaults

- Create `backend/internal/service/grok_account_defaults.go`: shared default mapping constant and merge helper.
- Create `backend/internal/service/grok_account_defaults_test.go`: explicit mapping wins and missing mapping defaults.
- Modify `backend/internal/service/account_service.go`: apply defaults during Grok account creation only.
- Modify `backend/internal/service/account_service_test.go`: create-service regression coverage.
- Modify `frontend/src/components/account/CreateAccountModal.vue`: initialize Grok mappings and preselect compatible Grok groups.
- Modify `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`: mapping, group selection, and platform-switch tests.

### Deployment

- Update `PRIVACYFILTER_WORKFLOW.md` or the existing custom-patch manifest with the new retained custom features.
- Create a versioned local build artifact under `build-artifacts/`.
- Back up `/opt/sub2api/sub2api`, the frontend assets, and PostgreSQL before deployment.
- Deploy through the existing systemd/Caddy layout and bind the production CC Switch key to groups `2, 12, 11` with group `2` as default.
- Assign Grok accounts `46-49` to group `12` through the normal service/API path or an equivalent transactional update plus scheduler cache refresh.

---

### Task 1: Add The API Key Group Binding Schema

**Files:**
- Create: `backend/migrations/185_api_key_groups.sql`
- Create: `backend/migrations/api_key_groups_migration_test.go`
- Create: `backend/ent/schema/api_key_group.go`
- Modify: `backend/ent/schema/api_key.go`
- Modify: `backend/ent/schema/group.go`
- Test: `backend/internal/repository/migrations_schema_integration_test.go`

- [ ] **Step 1: Write the failing migration contract test**

Assert that the migration creates the composite primary key, foreign keys,
group lookup index, and backfill from non-null `api_keys.group_id`:

```go
func TestAPIKeyGroupsMigrationContract(t *testing.T) {
    sql := readMigration(t, "185_api_key_groups.sql")
    require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS api_key_groups")
    require.Contains(t, sql, "PRIMARY KEY (api_key_id, group_id)")
    require.Contains(t, sql, "SELECT id, group_id")
    require.Contains(t, sql, "WHERE group_id IS NOT NULL")
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `cd backend && go test ./migrations -run TestAPIKeyGroupsMigrationContract -count=1`

Expected: FAIL because the migration file does not exist.

- [ ] **Step 3: Add the additive migration**

Use this shape:

```sql
CREATE TABLE IF NOT EXISTS api_key_groups (
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (api_key_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_api_key_groups_group_id
    ON api_key_groups(group_id);

INSERT INTO api_key_groups (api_key_id, group_id)
SELECT id, group_id FROM api_keys
WHERE group_id IS NOT NULL AND deleted_at IS NULL
ON CONFLICT (api_key_id, group_id) DO NOTHING;
```

- [ ] **Step 4: Add the Ent edge schema and regenerate Ent**

Model `APIKeyGroup` after `AccountGroup`, using `field.ID("api_key_id", "group_id")`.
Add `api_key_groups` edges to `APIKey` and `Group` without removing the existing
single `group` edge.

Run the repository's existing Ent generation command from `backend` as documented
in `DEV_GUIDE.md`. Verify generated diffs only contain the expected API key group
entity and edge changes.

- [ ] **Step 5: Run migration and schema tests**

Run:

```bash
cd backend
go test ./migrations -run 'APIKeyGroups|Migration' -count=1
go test ./internal/repository -run 'Migration|Schema' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the schema slice**

```bash
git add backend/migrations backend/ent/schema backend/ent
git commit -m "feat: add API key group bindings"
```

### Task 2: Dual-Read And Dual-Write API Key Bindings

**Files:**
- Modify: `backend/internal/service/api_key.go`
- Modify: `backend/internal/service/api_key_service.go`
- Modify: `backend/internal/repository/api_key_repo.go`
- Modify: `backend/internal/repository/api_key_repo_test.go`
- Test: `backend/internal/repository/api_key_repo_integration_test.go` if present; otherwise add focused cases to the existing integration suite.

- [ ] **Step 1: Write failing service and repository tests**

Cover these invariants:

```go
func TestAPIKeySelectedGroupReturnsRequestLocalCopy(t *testing.T) {
    key := APIKey{GroupID: ptr(int64(2)), Groups: []Group{{ID: 2}, {ID: 12}}}
    selected, err := key.WithSelectedGroup(12)
    require.NoError(t, err)
    require.Equal(t, int64(12), *selected.GroupID)
    require.Equal(t, int64(2), *key.GroupID)
}
```

Repository tests must prove that create/update writes all bindings in one
transaction, legacy `group_id` becomes a one-row binding, and reads populate
`GroupIDs`/`Groups` in stable ID order.

- [ ] **Step 2: Run focused tests and confirm failure**

Run:

```bash
cd backend
go test ./internal/service -run 'APIKeySelectedGroup|APIKeyGroup' -count=1
go test ./internal/repository -run 'APIKey.*Group' -count=1
```

Expected: FAIL because the multi-group fields and persistence do not exist.

- [ ] **Step 3: Implement request-local selection**

Add fields and a non-mutating helper:

```go
type APIKey struct {
    // existing fields
    GroupID  *int64
    Group    *Group
    GroupIDs []int64
    Groups   []Group
}

func (k *APIKey) WithSelectedGroup(groupID int64) (*APIKey, error) {
    clone := *k
    for i := range k.Groups {
        if k.Groups[i].ID == groupID {
            group := k.Groups[i]
            clone.GroupID = &group.ID
            clone.Group = &group
            return &clone, nil
        }
    }
    return nil, ErrAPIKeyGroupNotBound
}
```

Use the project's structured error type rather than a package-global bare error
if service conventions require HTTP metadata.

- [ ] **Step 4: Implement transactional binding persistence**

Keep `api_keys.group_id` as the default. On create/update with `group_ids`, validate
the default is included, replace join rows in the same transaction, and enqueue
the existing auth cache invalidation event after commit. On legacy writes with
only `group_id`, replace bindings with exactly that group.

- [ ] **Step 5: Run service, repository, and cache invalidation tests**

Run:

```bash
cd backend
go test ./internal/service -run 'APIKey' -count=1
go test ./internal/repository -run 'APIKey|AuthCache' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the persistence slice**

```bash
git add backend/internal/service/api_key.go backend/internal/service/api_key_service.go backend/internal/repository
git commit -m "feat: persist multiple API key groups"
```

### Task 3: Add Deterministic Model-To-Group Routing

**Files:**
- Create: `backend/internal/service/api_key_group_router.go`
- Create: `backend/internal/service/api_key_group_router_test.go`

- [ ] **Step 1: Write the complete resolver test table**

```go
func TestResolveAPIKeyRequestGroup(t *testing.T) {
    tests := []struct {
        model    string
        platform string
    }{
        {"gpt-5.4", PlatformGrok},
        {"claude-opus-4-8", PlatformGrok},
        {"grok-4.5", PlatformGrok},
        {"gpt-5.5", PlatformOpenAI},
        {"gpt-5.6-sol", PlatformOpenAI},
        {"claude-fable-5", PlatformAnthropic},
    }
    // Each case binds groups 2/openai, 12/grok, and 11/anthropic and asserts ID.
}
```

Also test trimming/case folding, unknown-model default fallback, unbound selected
platform, inactive group, and two active groups for one non-default platform.

- [ ] **Step 2: Run the resolver tests and confirm failure**

Run: `cd backend && go test ./internal/service -run 'ResolveAPIKeyRequestGroup' -count=1`

Expected: FAIL because the resolver does not exist.

- [ ] **Step 3: Implement the pure resolver**

Use explicit aliases before prefixes:

```go
func requestedPlatform(model string) string {
    normalized := strings.ToLower(strings.TrimSpace(model))
    switch normalized {
    case "gpt-5.4", "claude-opus-4-8":
        return PlatformGrok
    }
    switch {
    case strings.HasPrefix(normalized, "grok-"):
        return PlatformGrok
    case strings.HasPrefix(normalized, "gpt-"):
        return PlatformOpenAI
    case strings.HasPrefix(normalized, "claude-"):
        return PlatformAnthropic
    default:
        return ""
    }
}
```

Select only active bound groups. Prefer the legacy/default group when multiple
groups share the selected platform; otherwise return a configuration error.

- [ ] **Step 4: Run resolver tests**

Run: `cd backend && go test ./internal/service -run 'APIKeyRequestGroup' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the resolver**

```bash
git add backend/internal/service/api_key_group_router.go backend/internal/service/api_key_group_router_test.go
git commit -m "feat: route API keys by requested model"
```

### Task 4: Resolve The Request Group Before Subscription And Dispatch

**Files:**
- Modify: `backend/internal/server/middleware/api_key_auth.go`
- Modify: `backend/internal/server/middleware/api_key_auth_google.go`
- Modify: `backend/internal/server/middleware/api_key_auth_test.go`
- Modify: `backend/internal/server/routes/gateway.go`
- Modify: `backend/internal/server/routes/gateway_test.go`

- [ ] **Step 1: Add failing middleware tests**

Build requests with one API key bound to groups 2/openai, 12/grok, and
11/anthropic. Assert downstream middleware observes:

```go
require.Equal(t, int64(12), *apiKey.GroupID) // body model gpt-5.4
require.Equal(t, service.PlatformGrok, apiKey.Group.Platform)
require.Equal(t, int64(2), *cachedKey.GroupID) // source object unchanged
```

Add one test where subscription lookup receives group 12 rather than the default
group 2. Add route tests proving `/v1/responses` and `/v1/messages` dispatch to
Grok for the explicit aliases and to their native handlers for the other model
families.

- [ ] **Step 2: Run middleware and route tests and confirm failure**

Run:

```bash
cd backend
go test ./internal/server/middleware -run 'APIKey.*GroupRouting' -count=1
go test ./internal/server/routes -run 'MultiGroup|ModelRouting' -count=1
```

Expected: FAIL because authentication still exposes only the default group.

- [ ] **Step 3: Add bounded body model extraction**

After the existing request body limit middleware, read and restore JSON bodies:

```go
body, err := io.ReadAll(c.Request.Body)
c.Request.Body = io.NopCloser(bytes.NewReader(body))
model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
```

Do not read bodies for methods that cannot carry a model. Preserve malformed-body
handling for the existing gateway handler; routing uses the default group when no
valid string model can be extracted.

- [ ] **Step 4: Select a request-local key before subscription resolution**

Call the pure resolver after key authentication and before the current subscription
lookup. Replace only the context value with the selected copy. Keep Google forced
routes restricted to a bound Google-compatible group.

- [ ] **Step 5: Run middleware, route, billing, and gateway tests**

Run:

```bash
cd backend
go test ./internal/server/middleware -count=1
go test ./internal/server/routes -count=1
go test ./internal/handler -run 'Gateway|Billing|Sticky|Usage' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the runtime routing slice**

```bash
git add backend/internal/server/middleware backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_test.go
git commit -m "feat: select API key group per request"
```

### Task 5: Extend API Key Contracts And Validation

**Files:**
- Modify: `backend/internal/handler/api_key_handler.go`
- Modify: `backend/internal/handler/api_key_handler_test.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/server/api_contract_test.go`

- [ ] **Step 1: Add failing create/update contract tests**

Assert these payload rules:

```json
{
  "name": "CC Switch",
  "group_id": 2,
  "group_ids": [2, 12, 11]
}
```

- legacy `group_id` alone remains valid
- `group_ids` are deduplicated
- default `group_id` must be included
- inaccessible/inactive group IDs are rejected atomically
- responses include sorted `group_ids` and complete public `groups`

- [ ] **Step 2: Run handler contract tests and confirm failure**

Run: `cd backend && go test ./internal/handler ./internal/server -run 'APIKey.*Group' -count=1`

Expected: FAIL because request/response DTOs expose one group.

- [ ] **Step 3: Implement dual contract support**

Add `GroupIDs []int64 \`json:"group_ids"\`` to create/update requests and response
DTOs. Normalize IDs before service calls. Preserve the distinction between an
omitted `group_ids` field and an explicitly empty list so updates do not clear
bindings accidentally.

- [ ] **Step 4: Run handler and API contract tests**

Run:

```bash
cd backend
go test ./internal/handler -run 'APIKey' -count=1
go test ./internal/server -run 'APIContract|APIKey' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the API contract slice**

```bash
git add backend/internal/handler/api_key_handler.go backend/internal/handler/api_key_handler_test.go backend/internal/handler/dto backend/internal/server/api_contract_test.go
git commit -m "feat: expose API key group bindings"
```

### Task 6: Build The Multi-Group API Key UI

**Files:**
- Modify: `frontend/src/types/index.ts` or the current API key type source.
- Modify: `frontend/src/api/keys.ts`
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/views/user/__tests__/KeysView.spec.ts`
- Modify: `frontend/src/components/__tests__/ApiKeyCreate.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/keys.ts`
- Modify: `frontend/src/i18n/locales/en/keys.ts`

- [ ] **Step 1: Add failing frontend tests**

Cover create and edit flows:

```ts
expect(keysAPI.create).toHaveBeenCalledWith(expect.objectContaining({
  group_id: 2,
  group_ids: [2, 11, 12],
}))
```

Assert at least one group is selected, exactly one selected group is marked
default, existing single-group keys hydrate correctly, and changing bindings does
not change quota, expiration, ACL, or rate-limit fields.

- [ ] **Step 2: Run focused frontend tests and confirm failure**

Run:

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/KeysView.spec.ts src/components/__tests__/ApiKeyCreate.spec.ts
```

Expected: FAIL because the UI and API payload are single-group.

- [ ] **Step 3: Implement the multi-select and default selector**

Reuse the project's checkbox/multi-select patterns. Display selected groups as
compact platform badges and use a radio control among selected groups for the
default. Do not place a second card inside the existing modal.

Serialize selected IDs in stable numeric order and send the selected default as
`group_id`.

- [ ] **Step 4: Run frontend tests and type checking**

Run:

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/KeysView.spec.ts src/components/__tests__/ApiKeyCreate.spec.ts
pnpm type-check
```

Expected: PASS.

- [ ] **Step 5: Commit the API key UI slice**

```bash
git add frontend/src/types frontend/src/api/keys.ts frontend/src/views/user/KeysView.vue frontend/src/views/user/__tests__/KeysView.spec.ts frontend/src/components/__tests__/ApiKeyCreate.spec.ts frontend/src/i18n
git commit -m "feat(frontend): configure API key group bindings"
```

### Task 7: Add Grok Account Creation Defaults

**Files:**
- Create: `backend/internal/service/grok_account_defaults.go`
- Create: `backend/internal/service/grok_account_defaults_test.go`
- Modify: `backend/internal/service/account_service.go`
- Modify: `backend/internal/service/account_service_test.go`
- Modify: `frontend/src/components/account/CreateAccountModal.vue`
- Modify: `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`

- [ ] **Step 1: Add failing backend default tests**

Assert missing mappings produce exactly:

```go
map[string]any{
    "claude-opus-4-8": "grok-4.5",
    "gpt-5.4":          "grok-4.5",
}
```

Explicit non-empty `credentials.model_mapping` must remain byte-for-byte
equivalent after normalization, and non-Grok platforms must not receive Grok
defaults.

- [ ] **Step 2: Add failing frontend creation tests**

When platform becomes `grok`, assert two mapping rows appear and all active Grok
groups are selected. Changing to another platform before save removes only the
unsaved Grok defaults. Returning to Grok restores them if the user has not entered
custom mappings.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
cd backend && go test ./internal/service -run 'GrokAccountDefaults' -count=1
cd ../frontend && pnpm vitest run src/components/account/__tests__/CreateAccountModal.spec.ts
```

Expected: FAIL because the defaults are absent.

- [ ] **Step 4: Implement backend defaults**

Use one immutable helper:

```go
func ApplyGrokCreateDefaults(credentials map[string]any) map[string]any {
    if _, explicit := credentials["model_mapping"]; explicit {
        return credentials
    }
    clone := maps.Clone(credentials)
    clone["model_mapping"] = map[string]any{
        "claude-opus-4-8": "grok-4.5",
        "gpt-5.4":          "grok-4.5",
    }
    return clone
}
```

Apply it only in the create path after platform validation and before repository
persistence.

- [ ] **Step 5: Implement frontend defaults**

Initialize the same two rows in `CreateAccountModal.vue` when Grok is selected.
Reuse the existing compatible-group selection helper introduced by commit
`694e5b621`; ensure Grok group `#12` is selected from live group data rather than
hardcoding the numeric ID.

- [ ] **Step 6: Run backend and frontend tests**

Run:

```bash
cd backend
go test ./internal/service -run 'GrokAccountDefaults|Account.*Create' -count=1
cd ../frontend
pnpm vitest run src/components/account/__tests__/CreateAccountModal.spec.ts
pnpm type-check
```

Expected: PASS.

- [ ] **Step 7: Commit the Grok defaults slice**

```bash
git add backend/internal/service frontend/src/components/account
git commit -m "feat: add Grok account creation defaults"
```

### Task 8: Full Regression, Build, And Custom Feature Manifest

**Files:**
- Modify: `PRIVACYFILTER_WORKFLOW.md` or the active custom feature manifest.
- Generated: production backend binary and frontend assets.

- [ ] **Step 1: Record the retained custom features**

Add entries for:

- API key multi-group bindings
- explicit alias and family routing rules
- Grok account default mappings
- automatic compatible Grok group selection

Keep the existing privacy filter, Any Router passthrough, account test, OpenAI
defaults, and compatible-group defaults documented as retained.

- [ ] **Step 2: Run backend formatting and full tests**

Run:

```bash
cd backend
gofmt -w \
  ent/schema/api_key_group.go ent/schema/api_key.go ent/schema/group.go \
  internal/service/api_key.go internal/service/api_key_service.go \
  internal/service/api_key_group_router.go internal/service/grok_account_defaults.go \
  internal/repository/api_key_repo.go \
  internal/handler/api_key_handler.go internal/handler/dto/types.go internal/handler/dto/mappers.go \
  internal/server/middleware/api_key_auth.go internal/server/middleware/api_key_auth_google.go \
  internal/server/routes/gateway.go
go test ./... -count=1
```

Expected: PASS with zero failing packages.

- [ ] **Step 3: Run frontend formatting, tests, and production build**

Run the scripts defined in `frontend/package.json`:

```bash
cd frontend
pnpm lint
pnpm type-check
pnpm vitest run
pnpm build
```

Expected: every command exits 0 and `dist/` is produced.

- [ ] **Step 4: Build the Linux backend artifact**

Use the repository's established release build command and embed the frontend
assets in the same way as the currently deployed custom binary. Record SHA256,
Git commit, and version string in `build-artifacts/` metadata.

- [ ] **Step 5: Review the complete diff**

Run:

```bash
git diff --check HEAD~7..HEAD
git status --short
git log --oneline -10
```

Expected: no whitespace errors, no untracked source artifacts, and only intended
feature commits.

- [ ] **Step 6: Commit manifest/build metadata**

```bash
git add PRIVACYFILTER_WORKFLOW.md build-artifacts/v161-multi-group-routing-build.txt
git commit -m "docs: retain multi-group routing customizations"
```

### Task 9: Back Up, Deploy, Bind Production Data, And Smoke Test

**Files and services:**
- Remote: `/opt/sub2api/sub2api`
- Remote: current frontend/static asset location used by Sub2API
- Remote: a new timestamped directory under `/opt/sub2api/backups/`
- Service: `sub2api.service`
- PostgreSQL database: `sub2api`

- [ ] **Step 1: Capture pre-deployment state**

Record service status, current binary SHA256/version, CC Switch API key ID/default
group, group IDs 2/11/12, Grok account IDs 46-49, and negative/active account
states. Do not print API key secrets or account credentials.

- [ ] **Step 2: Create verified backups**

Back up the binary, frontend assets, environment/service files, and a compressed
PostgreSQL dump. Verify backup files exist, are non-empty, and have restrictive
permissions.

- [ ] **Step 3: Deploy migration, frontend, and binary**

Upload to temporary remote paths, verify SHA256, atomically replace artifacts,
restart `sub2api`, and wait for the health endpoint and admin UI version to return.
If health does not recover, restore the backup immediately.

- [ ] **Step 4: Verify legacy key behavior before data changes**

Use existing single-group keys to make one read-only/model-list or minimal request
per current platform. Verify their selected group and billing logs remain unchanged.

- [ ] **Step 5: Bind the CC Switch key and Grok accounts**

Through the normal admin/user API, set the existing `CC Switch` key bindings to
`[2, 11, 12]` with default `group_id=2`. Assign accounts 46-49 to group 12. Read
back the key/account records and verify scheduler/auth cache invalidation events
were consumed.

- [ ] **Step 6: Run live routing smoke tests**

With the existing CC Switch key, send minimal streaming requests and verify logs:

```text
gpt-5.5          -> group 2  -> openai account
gpt-5.4          -> group 12 -> grok account -> upstream grok-4.5
claude-opus-4-8  -> group 12 -> grok account -> upstream grok-4.5
another claude-* -> group 11 -> anthropic account
```

Do not claim a route is successful unless the response status and usage/error log
both show the intended selected group and account platform.

- [ ] **Step 7: Verify UI defaults without creating unwanted accounts**

Open the add-account modal, select Grok, confirm the two mapping rows and Grok
group selection, then cancel the modal. No production account is created by this
verification.

- [ ] **Step 8: Final service and data integrity checks**

Verify:

- `systemctl is-active sub2api` returns `active`
- database integrity queries return one default plus three bindings for CC Switch
- accounts 46-49 belong to group 12
- no migration or panic errors appear in the post-deployment journal
- no CC Switch desktop database or conversation files were touched

Record the final binary SHA256, backup directory, deployed commit, and smoke-test
request IDs.
