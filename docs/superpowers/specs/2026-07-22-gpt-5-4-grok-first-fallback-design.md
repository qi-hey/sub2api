# GPT-5.4 Grok-First Account Fallback

## Goal

Keep the existing deterministic `gpt-5.4` route to the API key's bound Grok
group, where Grok accounts map `gpt-5.4` to `grok-4.5`. If that Grok account
pool is conclusively unavailable, retry the request through the same API key's
bound OpenAI group. Grok remains the preferred provider and normal Grok
concurrency waiting must not be converted into OpenAI spillover.

This is a v161 customization that must be retained in later upstream rebases.

## Scope

The behavior applies only to requests whose normalized requested model is
exactly `gpt-5.4`. It covers the OpenAI-compatible entry points used by the
gateway:

- `/v1/responses`
- `/v1/chat/completions`
- the `/v1/messages` compatibility bridge
- Responses WebSocket ingress before the first client-visible response frame

Other aliases and model families retain the current deterministic group route.
In particular, direct `grok-*` and `claude-opus-4-8` routes do not gain this
fallback.

## Route Order

For a new `gpt-5.4` request:

1. Resolve the existing request-local Grok group and run the current Grok
   scheduler and account mapping.
2. If Grok produces an account or a concurrency wait plan, remain in Grok.
3. If the Grok pool is conclusively unavailable before output starts, resolve
   one active OpenAI group bound to the same API key and retry there.
4. Inside the OpenAI group, retain the existing scheduler, account priority,
   model mapping, load balancing, failover, and waiting behavior.

The fallback group is never selected system-wide. It must be an active group
already bound to the authenticated API key. If no such group exists, or more
than one active OpenAI group is bound without a deterministic choice, return
the existing routing/configuration error.

## Fallback Eligibility

Fallback is allowed only before any response body or WebSocket response frame
has been sent and only when one of these account-pool conditions is true:

- there are no active and schedulable Grok accounts supporting `gpt-5.4`;
- every supporting Grok account is disabled, quota-limited, temporarily
  unschedulable, or excluded by prior failover attempts;
- every attempted Grok account failed with an existing failover-eligible
  credential, entitlement, rate-limit, transport, or upstream server error.

Fallback is not allowed for:

- a Grok concurrency wait plan or a merely busy account pool;
- invalid client requests or non-failover `4xx` responses;
- user, API key, subscription, or selected-group billing rejection;
- content moderation or security policy rejection;
- canceled/disconnected clients;
- any request after client-visible output has started;
- compact or endpoint capability errors that would not be valid on the
  fallback route.

This distinction prevents fallback from hiding request defects or bypassing
billing and access policy.

## Request-Local Routing State

The authenticated API key object remains immutable. The fallback resolver
creates a second request-local API key copy selected to the bound OpenAI group.
All group-sensitive work after the switch uses that copy:

- group and subscription billing eligibility;
- group rate multipliers and quota platform;
- account selection, concurrency slots, and failover exclusions;
- sticky-session keys and previous-response bindings;
- model restrictions and account-level mappings;
- usage records, operations telemetry, and error logs.

Failed Grok account IDs and OpenAI account IDs are tracked per route because
account exclusions must not accidentally change the candidate set of another
group. Route-switch count is separate from same-group account-switch count and
is limited to one Grok-to-OpenAI transition.

## Session Continuity

Grok is tried first for a new session. Once a session or `previous_response_id`
has successfully fallen back to OpenAI, subsequent requests for that context
remain in the OpenAI group until the existing sticky binding expires or is
cleared. This prevents encrypted reasoning, server-side response state, and
tool continuations from moving between incompatible providers.

A failed fallback attempt does not create a sticky binding. A later independent
request therefore starts with Grok again.

## Error Handling

If both routes are exhausted before output starts, return the most useful final
error while preserving diagnostic context for both route attempts. Existing
sanitization and error-passthrough rules remain authoritative. Logs add the
primary and fallback group/platform plus a fallback reason, but never include
credentials or full request bodies.

If the request cannot be safely replayed, the gateway returns the primary Grok
error and does not attempt OpenAI. Request bodies are reused through the
existing buffered/replayed body and mapped-body helpers rather than decoded and
re-encoded ad hoc.

## Alternatives Rejected

### Mixed Cross-Platform Scheduler Pool

Putting Grok and OpenAI accounts into one scheduler would be smaller, but it
would mix group billing, permission, sticky-session, and usage scopes. It also
cannot preserve the requirement that busy Grok accounts wait instead of
spilling over.

### Global Account Fallback

Searching every system account would let one API key reach groups it is not
bound to. This violates the existing multi-group authorization model.

### Priority-Only Configuration

Duplicating accounts or changing numeric priorities cannot express the
difference between busy, unavailable, and failover-eligible errors and would
not correctly switch group-sensitive billing state.

## Verification

Automated tests must prove:

- exact `gpt-5.4` requests select Grok first and use `grok-4.5` upstream;
- a Grok wait plan does not attempt OpenAI;
- no schedulable Grok account selects a bound OpenAI account;
- exhausted failover-eligible Grok errors switch once to OpenAI;
- non-failover `400`, billing rejection, cancellation, and started streaming do
  not switch groups;
- fallback is rejected when OpenAI is unbound or ambiguous;
- fallback uses OpenAI group billing, rate, sticky, concurrency, usage, and log
  identifiers;
- a session that successfully fell back remains in OpenAI;
- other models retain existing group routing and failover behavior;
- HTTP Responses, chat completions, Messages compatibility, and WebSocket
  ingress follow the same eligibility rules.

Production smoke tests use the existing CC Switch key. One healthy Grok account
must receive a normal `gpt-5.4` request first. A controlled temporary
unschedulable state for all Grok accounts must then route a new test session to
the bound OpenAI group without changing or deleting account configuration.
