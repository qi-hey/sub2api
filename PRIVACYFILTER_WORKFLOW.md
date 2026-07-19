# Privacyfilter Update Workflow

This repository keeps the Sub2API upstream source plus downstream customizations
that must survive every upstream update.

## Required downstream customizations

### OpenAI new-account model defaults and fallback mapping

This is a required downstream feature, introduced in commit `0d1d9f9d`.

Every newly created OpenAI account defaults to:

- Allowed models: `gpt-5.5`, `gpt-5.6-luna`, `gpt-5.6-sol`, `gpt-5.6-terra`.
- Primary mappings: `gpt-5.4 -> gpt-5.5` and
  `gpt-5.4-mini -> gpt-5.5`.
- Fallback mappings: `gpt-5.4 -> gpt-5.6-sol` and
  `gpt-5.4-mini -> gpt-5.6-sol`.

Duplicate source rows are persisted separately because JSON objects cannot
contain duplicate keys:

```json
{
  "model_mapping": {
    "gpt-5.4": "gpt-5.5",
    "gpt-5.4-mini": "gpt-5.5"
  },
  "model_mapping_fallbacks": {
    "gpt-5.4": ["gpt-5.6-sol"],
    "gpt-5.4-mini": ["gpt-5.6-sol"]
  }
}
```

On an upstream failover error, the OpenAI Responses and Messages handlers retry
the same account with the next fallback before switching accounts. Exhausted
fallbacks must not loop. Accounts without fallback configuration retain the
upstream pool-mode retry behavior.

Upgrade acceptance checklist:

- The Create Account modal shows the four allowed models and four mapping rows.
- The Edit Account modal round-trips `model_mapping_fallbacks` without collapsing rows.
- Scheduler snapshots retain `model_mapping_fallbacks`.
- Spark shadow credential filtering permits `model_mapping_fallbacks`.
- Existing account credentials are not bulk-modified; defaults apply only to new accounts.
- Frontend model whitelist tests and backend fallback tests pass.

#### Any Router deployment policy

This is also a required downstream operational setting. Any Router currently
advertises the exact model ID `gpt-5.6-sol`; do not assume that
`gpt-5.6-luna`, `gpt-5.6-terra`, or a bare `gpt-5.6` are available unless its
`/v1/models` response adds them.

The active Any Router account must retain:

- Direct mapping: `gpt-5.6-sol -> gpt-5.6-sol`.
- Primary mapping: `gpt-5.4 -> gpt-5.5`.
- Primary mapping: `gpt-5.4-mini -> gpt-5.5`.
- Fallback: `gpt-5.4 -> gpt-5.6-sol`.
- Fallback: `gpt-5.4-mini -> gpt-5.6-sol`.

Keep `gpt-5.5` as the preferred target. `gpt-5.6-sol` is both directly
requestable and the same-account fallback. Do not alter Any Router priority or
activate duplicate disabled Any Router accounts while restoring this setting.

After an upstream upgrade, database restore, or account import, verify both the
database credentials and Redis scheduler metadata (`sched:meta:<account_id>`)
contain `model_mapping_fallbacks`. Cache refresh must not require a Sub2API
restart.

### Embedded request privacy filter

The embedded privacy filter is a required downstream feature. It scans the
OpenAI-compatible `messages` or `input` request content before forwarding and
redacts detected secrets with the bundled gitleaks rules. Non-JSON bodies and
payloads without supported content fields remain unchanged.

Upgrade acceptance checklist:

- The shared gateway, Responses, Chat Completions, and OpenAI gateway entry
  points still invoke `RedactPrivacyRequestBody` before upstream forwarding.
- The embedded `privacy_filter_rules/gitleaks.toml` remains in the backend
  binary; deployment does not depend on a separate rules file.
- Privacy-filter unit tests and gateway integration tests pass.

### Any Router Codex passthrough and account tests

The `accounts.extra.openai_passthrough` setting and its create, edit, and bulk
edit controls are required downstream behavior. For pool-mode API-key accounts
under `anyrouter.top`, requests mapped to `gpt-5.5` retain the Codex-compatible
request shape, headers, encrypted-reasoning include, prompt cache key, and
unsupported-field cleanup.

When a client sends a non-streaming request through this Any Router path, the
upstream request is forced to stream and Sub2API bridges the result back to the
client's requested response mode. Recognized invalid-Codex, capacity, and
transient processing errors remain eligible for pool failover. The account test
endpoint must use the same request shape and headers as live traffic.

Upgrade acceptance checklist:

- The frontend round-trips `extra.openai_passthrough` without relying on the
  legacy `openai_oauth_passthrough` field.
- Streaming and non-streaming Any Router passthrough tests pass.
- Any Router account tests exercise the Codex-compatible payload instead of a
  generic OpenAI payload.
- Accounts not under `anyrouter.top`, non-pool accounts, and models other than
  the exact configured target keep their upstream behavior.

### Compatible default groups for new accounts

In standard mode, the Create Account modal preselects every active group whose
platform matches the selected account platform. Antigravity mixed scheduling
also includes its compatible Anthropic and Gemini groups. Group IDs always come
from live group data and are never hardcoded.

Changing platform refreshes untouched defaults. Manual selection or deselection
is preserved while the group list refreshes. Simple mode continues to omit
explicit group bindings.

### API key multi-group routing

API keys can bind multiple groups through the additive `api_key_groups` table
while `api_keys.group_id` remains the default and legacy-compatible group.
Create and update payloads accept both `group_id` and `group_ids`; omitted
`group_ids` keeps single-group behavior, while an explicit empty list or a
default group outside the bindings is rejected atomically.

One request-local group is selected before authorization, subscription checks,
billing, scheduler selection, sticky sessions, security checks, and usage
logging. The cached API key object is not mutated. Explicit aliases take
precedence over family routing:

```text
gpt-5.4          -> Grok
claude-opus-4-8 -> Grok
grok-*           -> Grok
other gpt-*      -> OpenAI/Codex
other claude-*   -> Anthropic/Claude
unknown models   -> default group
```

The key management UI exposes multi-select bindings plus one selected default.
Existing keys hydrate their legacy `group_id` as a one-item binding. ACL,
quota, rate-limit, expiration, and usage fields must not change when bindings
are edited.

### Grok new-account defaults

New Grok accounts allow `grok-4.5` directly and default to these compatibility
mappings:

```text
grok-4.5        -> grok-4.5
claude-opus-4-8 -> grok-4.5
gpt-5.4         -> grok-4.5
```

The direct mapping is required because explicit account mappings also act as
the scheduler whitelist. The Messages compatibility path resolves Claude
aliases to `grok-4.5` before account selection.

The frontend selects all compatible Grok groups from live data and submits both
the mappings and selected group IDs. Both backend account-creation services add
the mappings when callers omit `credentials.model_mapping`; explicit caller
configuration always wins. Existing accounts and edit operations are not
modified.

Upgrade acceptance checklist:

- Multi-group migration, repository, service, middleware, handler contract, and
  frontend key-management tests pass.
- Exact alias and family routing tests verify the selected group without
  cross-group failover.
- Grok create tests verify frontend payloads, both backend creation paths, and
  direct `grok-4.5` scheduler eligibility.
- Existing single-group API keys retain their original behavior.

### Grok Forbidden account maintenance

The account status filter includes a downstream-only `forbidden` value for
Grok accounts. It does not query `accounts.status`; it matches accounts whose
`extra.grok_usage_snapshot.status_code` is `403`. Selecting this filter in the
admin UI automatically selects the Grok platform.

The filtered view exposes a protected "delete all Forbidden" action so cleanup
is not limited to the current 20-row page. This action must retain all of the
following safeguards:

- It is visible only while the Grok Forbidden filter is active and the server
  reports at least one match.
- The UI sends the complete filter snapshot and the displayed total in one
  request. It never deletes accounts page by page.
- The server accepts only `platform=grok` and `status=forbidden`, resolves the
  filter again, and returns HTTP 409 without deleting anything if the live
  count differs from the confirmed count.
- At most 5,000 accounts can be deleted in one request. Deletion reuses the
  normal account cleanup path so group links, scheduled tests, and scheduler
  cache entries are also removed.
- Partial failures are reported with success and failure counts. There is no
  scheduled or automatic Forbidden deletion; an administrator must confirm it
  in the UI.

Upgrade acceptance checklist:

- A Grok account with a 403 usage snapshot appears in the Forbidden filter
  even when its primary account status remains `active`.
- Non-Grok accounts and Grok accounts without a 403 usage snapshot do not
  appear in this filter.
- Success, count-change (HTTP 409), and partial-failure frontend tests pass.
- Deployment verification remains read-only and never invokes the production
  deletion endpoint.

Branches:

- `upstream-clean`: official Sub2API source without local changes.
- `privacyfilter-v137`: the original privacyfilter patch extracted from the VPS build.
- `main`: current deployable branch.
- `deploy`: alias branch for the current deployable branch.

Update to a new upstream tag:

```powershell
.\scripts\update-upstream.ps1 v0.1.138
```

Then verify and push:

```powershell
git status
git log --oneline -5
git push origin main deploy --tags
```

The VPS backup with secrets, database dump, Caddy config, and systemd config is stored outside this Git repository under:

```text
D:\Codex\Codex-VPS-Sub2api\backups
```

Do not commit `sub2api.env`, database dumps, admin credentials, or Cloudflare tokens.
