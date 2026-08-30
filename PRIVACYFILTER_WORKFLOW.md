# Privacyfilter Update Workflow

This repository keeps the Sub2API upstream source plus downstream customizations
that must survive every upstream update.

## Current downstream release

The current downstream base is upstream `v0.1.183`, released as
`0.1.183-r68`. The upgrade retains all required downstream customizations in
this document and the upstream fixes accumulated through `v0.1.182` and
`v0.1.183`, including:

- OpenAI OAuth passthrough input normalization (`851436c55`, `3e26dfa5b`);
- OpenAI incomplete-stream proxy quarantine (`47ad29db3`);
- Grok HTTP 402 scheduling exclusion (`ca0d3314c`).
- Responses Lite normalization across OAuth, API-key, HTTP, and WebSocket paths;
- OpenAI quota-exhaustion 429 scheduling with reset-aware recovery;
- Codex `session-id` affinity without persistent-binding spillover;
- restored Responses custom-tool item ID type preservation;
- Kimi concurrency 403 temporary cooldown and failover;
- OpenAI image prompt preservation, Anthropic cache billing correction, and
  channel-monitor composite attribution fixes.
- Grok Responses function-tool schemas normalize mixed `oneOf`/`anyOf` roots
  to object-only variants; only a tool with no callable object variant is
  removed, so newer Codex `automation_update` schemas cannot reject the whole
  request before sampling.
- Grok object-root normalization resolves and safely inlines reachable local
  `$ref`/`$defs` branches before forwarding. This covers Codex Desktop schemas
  whose root unions reference nested object unions while preserving primitive
  nullable definitions used only by object properties.
- Referenced object unions nested directly inside another root union are
  flattened into direct object variants. xAI accepts those variants but rejects
  a root branch that is itself another `oneOf`/`anyOf`, even when every nested
  variant is an object.

No database migration was added between upstream `v0.1.181` and `v0.1.183`.

### r68 real Codex reasoning replay fix

Branch `custom/v183-r68` supersedes r67. A synthetic long-session test passed
on r67, but a real Codex Desktop request still returned 422 because its
historical reasoning item carried an `rs_*` ID. Removing only encrypted content
left a stale reasoning shell that xAI's ModelInput decoder rejected.

r68 converts visible reasoning summaries into ordinary conversation-summary
messages and removes the whole stale reasoning item after history reduction.
It also records a bounded xAI error-body detail in Ops logs so future 422
responses identify the exact rejected field without recording request content.

Before r67, r66
correctly reduced a 2 MB Grok request from about 484,000 to 467,000 estimated
tokens in roughly one second, but xAI returned 422 because the shortened full
history still carried opaque reasoning replay state and a stale
`previous_response_id` generated against the original history.

The release retains the complete r66 and r65 customizations
without importing or replacing it with another fork:

- the UI exposes only `off` and the recommended account-balanced `session`;
- legacy stored `device` and `full` values normalize to `session`;
- each enabled OpenAI OAuth-like account keeps its own persistent seed;
- account-balanced mode stabilizes installation/session IDs, derives a stable
  thread/window per real client session, and keeps turn IDs and timestamps
  fresh per request;
- default prompt cache keys are rewritten to the derived thread ID, not one
  account-global session key; explicit custom cache keys remain untouched;
- regular HTTP, passthrough, and WebSocket paths continue sharing one computed
  fingerprint snapshot;
- legacy `/responses/compact` bodies remain untouched for compatibility, while
  their outbound headers now use the same account-balanced fingerprint;
- new OpenAI OAuth and Setup Token accounts default to `session`;
- migration `231_default_existing_openai_codex_fingerprint_balanced.sql`
  changes all existing active OpenAI OAuth/Setup Token accounts to `session`
  once and creates missing seeds;
- after migration, administrators can manually select `off`; that choice is
  not periodically or on every restart forced back to `session`.
- Grok prompt-budget trimming now tokenizes fixed instructions and multi-megabyte
  tool schemas once, caches each input item's token contribution, removes only
  complete old turns/tool pairs, and rebuilds the request once. A production
  2 MB Codex request previously spent about 111 seconds rescanning the same
  schema 143 times and was canceled before any upstream request was made.
- after Grok prompt reduction, opaque reasoning/compaction replay state is
  removed while visible summaries are retained;
- a stale `previous_response_id` is removed only when every remaining tool
  output has a matching tool call or item reference in the retained input;
- an unpaired tool output keeps `previous_response_id`, preventing the recovery
  path from creating a new "No tool call found" failure;
- ordinary below-budget requests and the existing same-account encrypted
  reasoning retry behavior remain unchanged.
- real Codex `reasoning` items with stale `rs_*` IDs are removed after their
  visible summaries have been preserved as ordinary context messages;
- Grok upstream error events include the configured bounded response-body
  detail, while request bodies and credentials remain excluded.

For Grok 402, retain the downstream deterministic `schedulable=false` policy;
do not regress to upstream's temporary cooldown-only behavior. Upstream
composite groups are additive and must not replace API-key-bound OpenAI-to-Grok
runtime fallback.

## Required downstream customizations

### Codex account-balanced fingerprint convergence

This is a required downstream feature. The supported administrator-facing
`codex_fingerprint_mode` values are `off` and `session`. Legacy `device` and
`full` inputs must normalize to `session` for upgrade compatibility and must
not appear in the frontend.

The `session` value is the default balanced policy for newly created OpenAI
OAuth-like accounts:

- `codex_fingerprint_seed`: one system-managed UUID per account, preserved
  across edits and disable/re-enable cycles;
- `installation_id` and `session_id`: stable per account;
- `thread_id`: deterministically derived from the account seed plus the
  original client session;
- `window_id`: derived from that thread;
- `turn_id`: a fresh UUIDv7 for every request;
- `turn_started_at_unix_ms`: the real request time;
- default `prompt_cache_key`: rewritten to the derived thread ID;
- explicit non-default `prompt_cache_key`: preserved.

Account duplication must mint a new seed. User-supplied seeds must be ignored.
Missing/invalid modes on new accounts default to `session`. Migration 231
converts all existing active OpenAI OAuth-like accounts to `session` once.
Enabling the mode through full edit, key-level update, or bulk update must
create a missing seed atomically. A later manual `off` remains off because
completed migrations are not rerun.

All HTTP, passthrough, and WebSocket paths must share the same resolved IDs.
Compact request bodies must not receive normal Responses `client_metadata`
rewrites, but compact outbound headers must still converge. The UI must expose
only `Off` / `关闭` and `Account balanced (recommended)` /
`账号级平衡（推荐）`.

Upgrade acceptance checklist:

- backend fingerprint, passthrough, compact, and WebSocket parity tests pass;
- same account and client session keep installation/session/thread/window
  stable while consecutive turns receive different turn IDs;
- different client sessions under one account receive different thread/window
  IDs;
- session-mode default cache keys resolve to thread IDs;
- compact bodies remain unchanged while compact headers converge;
- frontend create/edit/bulk selectors expose only `off` and `session`;
- migration 231 updates OAuth and Setup Token accounts but leaves API-key and
  non-OpenAI accounts unchanged;
- after migration, an administrator can manually switch an account off.

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
claude-opus-4-8 -> Grok
grok-*           -> Grok
gpt-*            -> OpenAI/Codex
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
gpt-5.2         -> grok-4.5
gpt-5.4         -> grok-4.5
gpt-5.4-mini    -> grok-4.5
gpt-5.5         -> grok-4.5
gpt-5.6-luna    -> grok-4.5
gpt-5.6-sol     -> grok-4.5
gpt-5.6-terra   -> grok-4.5
```

The direct mapping is required because explicit account mappings also act as
the scheduler whitelist. The Messages compatibility path resolves Claude
aliases to `grok-4.5` before account selection.

The frontend selects all compatible Grok groups from live data and submits both
the mappings and selected group IDs. Both backend account-creation services
merge missing defaults into every new Grok account, including OAuth, SSO,
manual, bulk, and remote imports. Explicit caller values for the same source
model win. Migration `186` adds missing aliases to existing Grok accounts
without overwriting account-specific mappings.

Grok account concurrency defaults to `2` and must never exceed `2`. The create
and edit forms clamp Grok values to that limit. Both backend creation services,
single-account updates, duplication/import paths, and repository bulk updates
enforce the same rule for OAuth and API-key accounts. Existing value `1` is
preserved; missing, non-positive, or greater-than-two values normalize to `2`.
Migration `187` repairs existing rows and adds a database constraint so a
future bypass cannot persist an unsafe Grok concurrency value. Other platforms
retain their configured concurrency unchanged.

Upgrade acceptance checklist:

- Multi-group migration, repository, service, middleware, handler contract, and
  frontend key-management tests pass.
- Exact alias and family routing tests verify the selected group without
  cross-group failover.
- Grok create tests verify frontend payloads, both backend creation paths, and
  direct `grok-4.5` scheduler eligibility.
- Grok concurrency tests verify default `2`, maximum `2`, preservation of `1`,
  bulk-update enforcement, and the database constraint.
- Existing single-group API keys retain their original behavior.

### OpenAI-first bound-group Grok fallback

Requests for `gpt-5.2`, `gpt-5.4`, `gpt-5.4-mini`, `gpt-5.5`,
`gpt-5.6-luna`, `gpt-5.6-sol`, and `gpt-5.6-terra` use the API key's OpenAI
group first. If that route is conclusively unavailable before client-visible
output starts, the same request may switch once to that API key's uniquely
bound active Grok group, where the account mapping sends the request upstream
as `grok-4.5`. It never searches globally available groups or accounts.

An API key without a uniquely bound active Grok group is not fallback-eligible;
the original OpenAI no-account or upstream error is returned normally. Normal
OpenAI concurrency waiting does not trigger fallback. Fallback is allowed only
after no schedulable OpenAI account remains, or after failover-eligible OpenAI
credential, entitlement, rate-limit, transport, or upstream-server errors have
exhausted the OpenAI route. Invalid `400` client requests, billing or policy
rejections, cancellation, and requests that already emitted HTTP/SSE/WebSocket
output do not switch groups.

The route switch uses a request-local API key copy. The Grok group's
subscription, group RPM, scheduler, account slots, channel mapping, security
policy, sticky session, quota platform, usage record, and operations context
must all use that copy. The authenticated cached key remains unchanged. The
user-global RPM counter is not incremented a second time during a mid-request
route switch.

A successful Grok fallback binds the existing session hash or
`previous_response_id` to the Grok group. Later requests in that context
restore Grok before billing and scheduling so response continuity is not
broken. Failed fallback attempts do not create continuity bindings.

This customization covers:

- `/v1/responses` over HTTP;
- `/v1/chat/completions`;
- the `/v1/messages` compatibility bridge;
- Responses WebSocket ingress before its first downstream frame.

Upgrade acceptance checklist:

- A healthy OpenAI account receives each configured `gpt-*` request before Grok.
- An OpenAI account wait plan waits or rejects normally without attempting Grok.
- No OpenAI candidate and exhausted failover-eligible OpenAI errors switch once
  to the uniquely bound Grok group.
- OpenAI `400`, client cancellation, billing rejection, and started output never
  switch groups.
- Missing or ambiguous Grok bindings preserve the original OpenAI error.
- Grok fallback usage, subscription, concurrency, policy, quota, and logs use
  the Grok group ID and record `upstream_model=grok-4.5`.
- A successful fallback session remains on Grok on its next request.
- Other models preserve deterministic multi-group routing.
- HTTP Responses, Chat Completions, Messages, and Responses WebSocket tests pass.

### Group-wide proxy binding tool

R19 adds `More Actions -> Tools -> Bind Proxy by Group` to account management.
The administrator selects one group and one active proxy, sees the server
reported `account_count`, and confirms a warning that existing account proxies
will be overwritten. The operation targets the complete group regardless of
the account table's current page.

The frontend calls the existing filtered bulk-update contract with a ten-minute
client timeout:

```json
{
  "filters": { "group": "<group-id>" },
  "proxy_id": 123
}
```

The selector never offers the no-proxy option in this workflow. On completion,
the account list, active proxy list and group account counts are refreshed.
Future upstream upgrades must retain the typed API wrapper, modal, menu entry,
translations and tests.

### Grok outbound custom-tool history compatibility

Grok Responses forwarding must lower Codex-only custom-tool history even when
the current request omits `tools` or sends an empty tool list. Before sending a
request to xAI, remaining `custom_tool_call` items become `function_call` items
with their freeform input preserved in `arguments`, and
`custom_tool_call_output` items become `function_call_output` items.

This cleanup is Grok-outbound only. Historical tools must not be added to the
current turn's reversible client-tool mapping, and native Grok
`function_call`/`function_call_output` items, account model mappings, ordinary
messages, and reasoning items must remain unchanged.

Upgrade acceptance checklist:

- Missing and empty `tools` both lower custom-tool history in `input`.
- Retired historical tools are lowered without entering the current mapping.
- IDs, call IDs, names, freeform input, outputs, and ordinary messages survive.
- Native Grok function-tool history is a no-op.
- Grok protocol, service, and OpenAI-to-Grok fallback tests pass.

### Grok long-context prompt budgeting

Grok Responses prompt budgeting must retain the 468,000-token safety budget,
latest turn, fixed instructions, current tools, and complete tool call/output
pairs. When old history must be removed, fixed instructions and tool schemas
are tokenized once and each input item's contribution is cached. The trimmer
must not re-marshal and re-tokenize the full request after every removed turn.

This requirement applies to native Responses, Anthropic-to-Grok conversion,
and the WebSocket HTTP bridge because all three paths share
`applyGrokResponsesPromptBudget`.

Upgrade acceptance checklist:

- Cached per-item totals exactly match the existing full-request estimator.
- String input and below-budget requests remain byte-for-byte unchanged.
- Old complete turns and complete tool sets are removed without splitting a
  tool call from its output.
- A production-scale request larger than 1.5 MB can remove at least 140 turns
  within five seconds in the backend regression test.
- Requests whose latest turn alone exceeds the safe budget still return the
  existing explicit compact/new-session error.
- Any history reduction invalidates opaque replay state derived from the
  original history. Preserve visible summaries, remove encrypted replay data,
  and remove `previous_response_id` only when retained tool context is
  self-contained.

### Grok Forbidden account maintenance

The account status filter includes a downstream-only `forbidden` value for
Grok accounts. It does not query `accounts.status`; it matches accounts whose
`extra.grok_usage_snapshot.status_code` is `403`. Selecting this filter in the
admin UI automatically selects the Grok platform.

An active Grok quota probe or live forwarding response that receives the
structured xAI error `permission_denied` together with
`Access to the chat endpoint is denied` automatically sets the account to
`schedulable=false`. Live forwarding must persist an `upstream_response` 403
snapshot even when xAI omits quota headers. The 403 snapshot remains the source
of the Forbidden filter, so the disabled account can still be reviewed,
reauthorized, or deleted by an administrator. Ambiguous 403 responses and 429
rate limits must not permanently disable scheduling, and successful probes must
not automatically re-enable accounts that were disabled manually.

An xAI `402 Payment Required` response is also deterministic account
unavailability. Active probes and live forwarding must set that account to
`schedulable=false`, while preserving the account and credentials for later
review. A `402` is not included in the Forbidden filter, which remains scoped
to persisted `403` snapshots.

Administrators can force a real chat-endpoint probe with
`GET /api/v1/admin/grok/accounts/:id/quota?probe=active`. This bypasses the
billing-probe short circuit and uses the same OAuth token, account proxy,
headers, model, and Responses payload as live Grok traffic. The operational
tool `scripts/grok_inventory_probe.py` applies this probe to every current
`active + schedulable` Grok account with bounded concurrency, a single-run
lock, JSONL details, and a JSON summary. It never deletes accounts or changes
groups and credentials. Production reports are stored in
`/opt/sub2api/probe-reports`.

The filtered view exposes a protected "delete all Forbidden" action so cleanup
is not limited to the current 20-row page. This action must retain all of the
following safeguards:

- It is visible only while the last successfully loaded request matches the
  current Grok Forbidden filter and the server reports at least one match.
- The UI sends the complete filter snapshot and the displayed total in one
  request. It never deletes accounts page by page.
- The server accepts only `platform=grok` and `status=forbidden`, resolves the
  complete ID set twice, and returns HTTP 409 without deleting anything if the
  confirmed count or ID set changes.
- At most 5,000 accounts can be deleted in one request. Deletion reuses the
  normal account cleanup path so group links, scheduled tests, and scheduler
  cache entries are also removed. Each account is locked and rechecked as an
  undeleted Grok 403 row in the deletion transaction before any cleanup occurs.
- Partial failures are reported with success and failure counts. There is no
  scheduled or automatic Forbidden deletion; an administrator must confirm it
  in the UI.

Upgrade acceptance checklist:

- A Grok account with a 403 usage snapshot appears in the Forbidden filter
  even when its primary account status remains `active`.
- A structured chat-permission 403 from the active quota probe disables
  scheduling while preserving the account and OAuth credentials.
- The same structured 403 from live Responses, Messages compatibility,
  Chat Completions bridge, WebSocket bridge, or media traffic immediately
  persists a Forbidden snapshot and disables scheduling instead of cycling the
  account through a temporary cooldown.
- Ambiguous 403 responses and 429 rate limits do not permanently disable the
  account.
- A 402 response disables scheduling in both active probes and live traffic.
- A forced active probe always calls the xAI Responses endpoint even when the
  billing endpoint reports authoritative quota data.
- The inventory tool records successes, rate limits, deterministic disables,
  and transient failures without deleting accounts.
- Non-Grok accounts and Grok accounts without a 403 usage snapshot do not
  appear in this filter.
- Success, count-change (HTTP 409), and partial-failure frontend tests pass.
- Deployment verification remains read-only and never invokes the production
  deletion endpoint.

Branches:

- `upstream-clean`: official Sub2API source without local changes.
- `privacyfilter-v137`: the original privacyfilter patch extracted from the VPS build.
- `custom/v183-r63`: current deployable downstream branch.
- `custom-v0.1.183-r63`: immutable source tag for the current downstream release.

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
