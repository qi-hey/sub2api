# API Key Multi-Group Routing And Grok Defaults

## Goal

Allow one existing API key to access the Codex, Grok, and Claude groups without
creating separate keys. Route each request to one deterministic group before
subscription checks, billing, account selection, sticky sessions, and usage
logging. Also make newly created Grok accounts immediately usable with the
requested default model mappings and Grok group assignment.

The initial production target is the existing `CC Switch` API key, bound to:

- Codex group `#2`
- Grok group `#12`
- Claude group `#11`

Existing API keys and their current single-group behavior must remain compatible.

## Confirmed Routing Rules

Routing is deterministic and case-insensitive after trimming the requested model.
The following explicit aliases take precedence over model-family inference:

| Requested model | Selected platform/group | Account-level upstream mapping |
| --- | --- | --- |
| `gpt-5.4` | Grok / `#12` | `grok-4.5` |
| `claude-opus-4-8` | Grok / `#12` | `grok-4.5` |

All other models use these family rules:

| Requested model family | Selected platform/group |
| --- | --- |
| `grok-*` | Grok / `#12` |
| `gpt-*` | Codex / `#2` |
| `claude-*` | Claude / `#11` |

For the first release, ambiguous or unknown models do not search every bound
group. They use the API key's legacy/default `group_id`. This preserves existing
behavior and prevents a typo from silently reaching a different provider.

If the API key is not bound to the group selected by a routing rule, the gateway
returns a model/group routing error. It must not silently fall back to an
unbound group.

## Data Model

Add an `api_key_groups` join table:

```text
api_key_id  bigint  not null
group_id    bigint  not null
created_at  timestamptz not null
primary key (api_key_id, group_id)
```

Foreign keys reference `api_keys` and `groups`, with cascading deletion for the
binding rows only.

Keep `api_keys.group_id` as the legacy/default group. It remains the fallback
for unknown models and allows older code, exports, and clients to continue
working during the upgrade.

The migration backfills one join-table row for every non-null existing
`api_keys.group_id`. It does not modify API key values, quotas, usage counters,
expiration, ACLs, subscriptions, or session records.

The service `APIKey` model gains `GroupIDs` and `Groups`, while `GroupID` and
`Group` continue to represent the selected/default group for compatibility.

## Request Routing

### Authentication and selection

API key authentication loads the key's complete group bindings. A new request
group resolver selects exactly one group from the requested model using the
confirmed rules above.

Selection must happen before middleware or handlers make decisions based on
`apiKey.GroupID` or `apiKey.Group`, including:

- route dispatch between Anthropic, OpenAI, and Grok handlers
- subscription lookup and billing eligibility
- group rate multiplier lookup
- model mapping and restriction lookup
- account selection and failover
- sticky-session lookup and binding
- security and content-moderation group scope
- usage and error logging

After selection, the request context receives a request-local API key copy whose
`GroupID` and `Group` are the selected group. The cached/authenticated API key
object must not be mutated because concurrent requests using the same key may
select different groups.

### Model extraction

The routing layer extracts `model` once from JSON request bodies and restores the
body for the existing handler. Existing body-size limits still apply. Endpoints
without a model, such as key billing information, models, or usage, use the
legacy/default group unless an endpoint has an explicit forced platform.

Forced platform routes remain authoritative. A forced platform must still match
one of the key's bound groups.

### Session and failover isolation

Sticky-session cache keys already include or receive a group ID. The selected
group must be used consistently so one client session cannot bind a Codex
request to a Grok or Claude account.

Failover remains inside the selected group. A Grok upstream failure must not
fall through to Codex or Claude merely because those groups share the API key.

## API And UI

API key create/update payloads accept `group_ids` while continuing to accept
legacy `group_id`:

- `group_ids` controls all bindings.
- `group_id` is the default group and must be present in `group_ids` when both
  are supplied.
- Requests that only send `group_id` keep current single-group behavior.
- Responses expose both `group_id` and `group_ids`.

The API key create/edit UI changes the group selector from single-select to
multi-select and marks one selected group as the default. Existing keys open
with their current group selected as both binding and default.

The initial deployment adds groups `#2`, `#12`, and `#11` to the existing
`CC Switch` key while preserving `#2` as its default group. This production data
change happens only after migration and routing tests pass and after a database
backup is created.

## Grok Account Defaults

When creating a Grok account, initialize the allowed model and compatibility
mappings with:

```text
grok-4.5        -> grok-4.5
claude-opus-4-8 -> grok-4.5
gpt-5.4          -> grok-4.5
```

The frontend still presents the last two entries as the two compatibility
mapping rows. The direct entry keeps `grok-4.5` in the effective scheduler
whitelist after the Messages bridge resolves a Claude alias before selection.

The frontend preselects every active compatible Grok group; with the current
production data this selects Grok group `#12` automatically.

The backend applies the same effective model mapping when a Grok account is
created through the API without an explicit model restriction/mapping.
Explicit caller configuration always wins. Backend group assignment remains explicit in API
requests; the UI supplies the automatically selected group IDs. This avoids an
API import silently attaching accounts to newly created groups.

Changing the platform away from Grok before saving clears the unsaved Grok
defaults and applies the existing defaults for the newly selected platform.

## Compatibility And Rollout

Rollout order:

1. Back up the production database and current binary.
2. Apply the additive join-table migration and backfill existing bindings.
3. Deploy backend and frontend with dual-read/dual-write compatibility.
4. Verify all existing single-group keys behave unchanged.
5. Bind the existing `CC Switch` key to groups `#2`, `#12`, and `#11`, retaining
   `#2` as default.
6. Assign the new Grok accounts to group `#12` through the normal account update
   service so scheduler caches and outbox events are refreshed.
7. Run live smoke tests for one model in each route family.

No CC Switch desktop configuration, conversation history, API key secret, user
quota, or usage record is reset.

## Error Handling

- No matching route rule: use the default group.
- Rule selects an unbound platform/group: return a clear `400` routing error.
- More than one bound active group for a selected platform: prefer the default
  group when it has that platform; otherwise return a configuration error rather
  than selecting nondeterministically.
- Selected group is inactive or deleted: return the existing unavailable-group
  error without cross-group fallback.
- Invalid `group_ids`, duplicate IDs, inaccessible groups, or a default group not
  included in the bindings: reject create/update atomically.

## Verification

Backend tests cover:

- migration backfill and rollback safety
- API key repository create/get/update/list with multiple groups
- legacy `group_id` request compatibility
- request-local selection without mutating cached API keys
- exact alias precedence for `gpt-5.4` and `claude-opus-4-8`
- family routing for `grok-*`, other `gpt-*`, and other `claude-*`
- unbound, ambiguous, inactive, and unknown-model behavior
- billing, subscription, model mapping, usage logs, sticky sessions, and failover
  all receiving the selected group ID

Frontend tests cover:

- API key create/edit multi-select and default-group validation
- legacy key display
- Grok account creation default mappings
- Grok compatible groups automatically selected
- explicit mappings are not overwritten
- changing away from Grok clears unsaved Grok defaults

Production smoke tests use the existing CC Switch key and verify:

- `gpt-5.5` selects Codex `#2`
- `gpt-5.4` selects Grok `#12` and maps to `grok-4.5`
- `claude-opus-4-8` selects Grok `#12` and maps to `grok-4.5`
- another available `claude-*` model selects Claude `#11`
- usage/error records contain the actually selected group
