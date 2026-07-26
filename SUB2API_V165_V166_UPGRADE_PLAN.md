# Sub2API v0.1.165/v0.1.166 Upgrade Plan

## Objective

Upgrade directly from the verified downstream v0.1.163 baseline to the newest
acceptable upstream tag (`v0.1.165` or `v0.1.166`) while retaining every
required customization in `PRIVACYFILTER_WORKFLOW.md`.

The upgrade must also retain these three valuable fixes first released in
upstream `v0.1.164`:

1. Normalize scalar or object `input` values on OpenAI OAuth passthrough paths.
2. Quarantine a failing OpenAI account proxy after repeated incomplete streams.
3. Remove Grok accounts that return HTTP 402 from active scheduling.

This document is a future execution plan. Creating it does not update source,
the database, the production binary, or the running service.

## Version Selection Gate

Do not automatically install the first newer tag.

1. Fetch the release notes, tag commit, migration list, and full diff for both
   `v0.1.165` and `v0.1.166` when they exist.
2. Prefer the newest tag that has no known blocker in OpenAI Responses, Grok,
   account scheduling, multi-group API keys, migrations, or embedded frontend.
3. If `v0.1.166` is available and contains all `v0.1.165` changes, skip the
   intermediate production deployment and rebase directly onto `v0.1.166`.
4. Do not upgrade solely for Ollama Cloud usage or mobile Alipay support, which
   are not required by the current deployment.
5. Treat upstream composite groups as optional. They may be included because
   they are part of the selected upstream tag, but they must remain disabled
   for the existing CC Switch key until the routing matrix below passes.

## Immutable Starting Point

Before merging any upstream code:

1. Query production for the running version, binary SHA256, service state, and
   database migration version.
2. Match that SHA256 to the local build artifact and deployment record.
3. Create an immutable source tag for the matching downstream release.
4. Save the current production binary and a PostgreSQL backup under a dated
   rollback directory on the VPS.
5. Export only non-secret configuration metadata needed for comparison. Never
   commit API keys, OAuth credentials, SSO tokens, database dumps, or env files.
6. Build in a new clean worktree. Do not merge from the currently dirty
   development worktree or assume its `HEAD` alone represents production.

The rollback directory must contain at least:

```text
sub2api binary
binary.sha256
version.txt
database dump
service unit/config metadata
health and migration baseline
```

## Upstream Integration Strategy

Use a clean upstream-tag merge, then restore downstream features by coherent
feature commits. Do not copy the old binary or overwrite upstream files in
bulk.

1. Create `upgrade/v0.1.16x-custom` from the verified downstream source tag.
2. Merge the selected upstream tag with conflicts left visible.
3. Resolve generated Ent and Wire files from source schema/provider changes,
   then regenerate them using the selected upstream toolchain.
4. Resolve the 44 known v0.1.164 overlap areas first: group routing, gateway
   routes, OpenAI passthrough, Grok forwarding, account repository, billing,
   account UI, and generated schema files.
5. Reapply downstream changes by feature, not by file, in this order:
   privacy filter; account defaults; multi-group keys; deterministic routing;
   OpenAI-to-Grok fallback; Grok protocol/history compatibility; Grok account
   safety and maintenance; Any Router passthrough; SSO maintenance; remaining
   UI and operations customizations.
6. Generate a custom version such as `0.1.166-r1`. Increment the downstream
   revision for every production redeploy of the same upstream tag.

## Required v0.1.164 Fixes

These fixes are expected to be inherited by v0.1.165/v0.1.166. Verify their
behavior after conflict resolution instead of blindly cherry-picking commits.

### OpenAI passthrough input normalization

Upstream origin: `851436c55` plus type cleanup `3e26dfa5b`.

Required behavior:

- string `input` becomes a one-item user message array;
- empty string becomes an empty array;
- object `input` becomes a one-item array;
- existing array input remains unchanged;
- unsupported internal fields and compact-specific cleanup still run;
- normalization applies only to the OAuth passthrough request path that needs
  it and does not rewrite ordinary pool-mode Any Router payloads incorrectly.

Required tests:

- string, empty string, object, array, missing input, compact and non-compact;
- Any Router `extra.openai_passthrough` stream and non-stream bridges;
- a real `/v1/responses` smoke request through an official OpenAI account.

### OpenAI stream proxy quarantine

Upstream origin: `47ad29db3`.

Required behavior:

- count only genuine incomplete upstream stream failures;
- do not count client cancellation or deadline cancellation;
- two failures within one minute quarantine the proxy for ten minutes, unless
  newer upstream defaults intentionally improve those values;
- a successful terminal stream clears the observation;
- quarantine is proxy-scoped, bounded, in-memory, and expires automatically;
- accounts without a proxy are not quarantined;
- scheduler diagnostics expose `proxy_stream_quarantined`.

Required tests:

- normal `[DONE]` and terminal event clear state;
- incomplete stream before and after first output;
- client cancellation does not trip the circuit;
- only accounts sharing the bad proxy are excluded;
- healthy direct OpenAI accounts remain selectable.

### Grok HTTP 402 scheduling policy

Upstream origin: `ca0d3314c`.

The downstream policy is intentionally stronger than upstream's 30-minute
cooldown. Preserve the current behavior:

- a deterministic Grok `402 Payment Required` from active probing or live
  traffic persists the usage snapshot and sets `schedulable=false`;
- the account and credentials remain available for administrator review;
- it is not deleted automatically and is not included in the 403 Forbidden
  filter;
- transient transport failures and unrelated 4xx responses do not disable it;
- repository/cache/scheduler state is refreshed immediately.

Do not replace this with the weaker upstream-only temporary cooldown during
conflict resolution.

## Composite Groups And Downstream Routing

Composite groups and downstream multi-group failover solve different problems.
Keep both layers separate:

- Composite group: deterministic model/endpoint alias to one concrete platform.
- Multi-group key: authorizes one key for several existing concrete groups.
- Downstream fallback: switches once from OpenAI to the same key's bound Grok
  group only after an eligible OpenAI failure and before output starts.

For the existing CC Switch key, keep the current concrete bindings and default
group. Do not migrate it to a composite group during the version upgrade.

If composite groups are enabled later, route resolution must happen first and
the fallback must be constrained to the concrete groups authorized by the same
key. It must never search global accounts or interpret route priority as
runtime failover priority.

Required routing matrix:

| Scenario | Expected result |
| --- | --- |
| Healthy OpenAI request for configured `gpt-*` | OpenAI group |
| OpenAI wait plan / busy concurrency | Wait or reject; no Grok spillover |
| OpenAI pool conclusively unavailable | One switch to uniquely bound Grok group |
| OpenAI client error, billing error, policy rejection | Original error; no switch |
| Output already started | No cross-platform replay |
| Missing or ambiguous Grok binding | Original OpenAI error |
| Established OpenAI session | Remain OpenAI |
| Established Grok fallback session | Remain Grok |
| Explicit `grok-*` | Grok directly |
| Explicit `claude-*` | Existing Claude/Grok alias policy, unchanged |
| Composite group static route | Exactly its configured target; no implicit fallback |

## Database And Migration Safety

1. List all migrations between `v0.1.163` and the selected tag and check for
   duplicate downstream migration numbers before building.
2. Rename downstream migrations if necessary while preserving already-applied
   production history; never edit an applied migration in place.
3. Test upgrade against a restored production-shaped database copy.
4. Verify `api_key_groups`, account mappings, group assignments, Grok
   concurrency constraints, usage snapshots, subscriptions, and session/sticky
   records survive unchanged.
5. Confirm a rollback to the old binary is database-compatible. If a new
   migration is not backward compatible, the rollback procedure must restore
   both the old binary and the pre-upgrade database backup.

## Automated Acceptance Suite

The release candidate cannot be deployed unless all of these pass:

```text
go test ./... -count=1
go vet ./...
frontend full Vitest suite
pnpm typecheck
pnpm lint:check
pnpm build
git diff --check
linux/amd64 embedded frontend build
```

Targeted regression suites must explicitly cover:

- embedded privacy filtering;
- OpenAI defaults and fallback mappings;
- Any Router passthrough, stream forcing, and account testing;
- multi-group API key authorization and request-local selection;
- all four OpenAI-to-Grok fallback ingress paths;
- conversation ownership and tool history compatibility;
- Grok concurrency maximum of two;
- Grok 402/403 persistence, filtering, disabling, and safe deletion;
- Grok default mappings and automatic group assignment for every import path;
- CC Switch import platform classification;
- the three inherited v0.1.164 fixes above;
- composite group isolation, even when production does not use it.

## Staging And Production Smoke Tests

Run against a production-shaped staging database first. Then deploy production
with one controlled restart and test in this order:

1. Local and public health endpoints return HTTP 200.
2. Admin UI loads, reports the expected custom version, and has no console
   errors.
3. Existing CC Switch key and its conversation history remain unchanged.
4. Official OpenAI `gpt-5.6-sol` returns HTTP 200 and records the OpenAI group.
5. A normal configured GPT request selects OpenAI before Grok.
6. A controlled no-account OpenAI condition switches to the bound Grok group
   and records `upstream_model=grok-4.5`.
7. A busy OpenAI condition does not spill to Grok.
8. A long tool-history request succeeds on its owning provider without losing
   call/result pairs.
9. Grok account concurrency never exceeds two.
10. Any Router stream and non-stream requests retain their expected behavior.

The controlled fallback test must use reversible temporary scheduling state.
It must not delete accounts, edit credentials, change API keys, or alter CC
Switch desktop configuration.

## Deployment And Rollback Gates

Deploy only when:

- candidate and uploaded binary SHA256 values match;
- database migration dry-run succeeded;
- all automated and staging tests passed;
- the rollback binary and database backup are present and readable;
- no unexplained routing, billing, or account-state diff remains.

Immediately roll back when any of these occurs:

- OpenAI requests cannot select a known healthy account;
- API key group bindings or session ownership change unexpectedly;
- client-visible output is replayed across providers;
- usage/billing is attributed to the wrong group or upstream model;
- service restart loop, panic, migration failure, or public health failure;
- Grok concurrency exceeds two;
- privacy filtering or Any Router passthrough regresses.

After deployment, write a new `build-artifacts/v16x-*.txt` record containing
source commit, upstream tag, feature list, test results, binary/archive SHA256,
production backup path, health results, and previous version.
