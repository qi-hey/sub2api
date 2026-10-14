# v0.2.16-r77 Upgrade Validation

## Baseline

- Date: 2026-10-14, Asia/Shanghai.
- Production version: 0.2.10-r76.
- Production and local r76 executable SHA256:
  `5e04dc45b10777b1fa0529cbe00221577b61f76f0436b57fdaf0593b07d6a7f7`.
- Source baseline: `d7d9c4899`, clean before the upgrade.
- Upstream target: `v0.2.16`.
- Downstream branch: `custom/v0216-r77`.

## Preservation

The fingerprint implementation and seed migrations are unchanged:

- `backend/internal/service/openai_codex_fingerprint.go`
- `backend/internal/service/openai_codex_identity.go`
- `backend/internal/service/openai_codex_account_identity.go`
- `backend/migrations/225_backfill_codex_fingerprint_seed.sql`
- `backend/migrations/231_default_existing_openai_codex_fingerprint_balanced.sql`

Production identity values are compared using hashes only. Reports, tests and
commits must not contain production seeds, credentials, or database contents.
No CC Switch configuration, provider selection, local proxy, model catalog,
reasoning setting, context-window override, or session record is modified.

## PR 7939

The upstream fix is included, without a separate enable switch. Tests cover
normal OAuth forwarding and passthrough, standard Responses and Responses Lite,
and the downstream account-balanced fingerprint for each combination. Empty
tool sets retain `tool_choice=none` and historical calls remain in the input.
Existing WebSocket and dedicated compact-endpoint regressions are retained.

## Backup and Rehearsal

- Server backup: `/opt/sub2api/backups/r76-before-r77-20261014T030519Z`.
- Local copy: workspace `backups/r76-before-r77-20261014T030519Z.tar`.
- Archive SHA256:
  `36d1ff7efdabd00a88b69dda5f86c20459863158b2bcc806ad313f1cb5839125`.
- The archive includes a PostgreSQL custom dump and globals, the old executable,
  application data/configuration, service unit/drop-ins, and reverse-proxy
  configuration. Private backups are outside this Git repository.
- Local CC Switch/Codex snapshot:
  `C:\Users\Administrator\.cc-switch\backups\before-sub2api-v0216-r77-20261014`.
- Database migration validation uses `cmd/verify-upgrade` against a restored
  database whose name must start with `sub2api_upgrade_verify_`.
- Staged application tests use independent databases, Redis and loopback ports,
  with restored upstream credentials scrubbed and external network access
  denied. The r76 executable and database restoration are also exercised.

The existing NAS backup timer reported a connection timeout before this work;
the independent upgrade backup above does not depend on that timer.

## Validation Status

- Frontend: 390 test files and 3006 tests passed.
- Frontend type checking and embedded production build passed.
- `go test -tags=unit ./...` passed for the complete backend.
- `go vet -tags=unit ./...` passed.
- The eight Forward-level PR 7939 cases include the four account-balanced
  fingerprint cases; all passed.
- Linux amd64 executable was built locally with embedded frontend assets.
- Candidate executable SHA256:
  `bdbe2e9b9697641d6ec8c9221402128acf773214df61bbfa99e150b8c414dba9`.
- Restored-database migrations passed: 306 applied migrations became 313.
  A second run was idempotent. Protected data digest was identical.
- Restoring the original database produced the same protected-state digest.
- Both r77 and the restored r76 passed isolated HTTP health/page/game-asset
  checks. The old r76 executable also started successfully on the migrated
  schema, so an immediate binary rollback need not discard new traffic data.
- Production now runs `0.2.16-r77`, with the candidate executable checksum above.
- Cutover backup: `/opt/sub2api/backups/r77-cutover-20261014T034500Z`.
  Its fresh database dump also has a verified local copy, SHA256
  `ac4b87abaa9eaea0a8531d50d8339b14044b45c80a0702d113df06f6773036cc`.
- Fingerprint, account-routing, account-group, key-group and group-settings
  hashes matched across the cutover. Application/systemd/Caddy configuration
  hashes were unchanged.
- The live canary used `gpt-6.1-sol`, a synthetic historical `web_search_call`,
  and `tools: []`. One attempt returned HTTP 200, `response.completed`, the
  expected `R77-7939-OK` marker, and zero new tool calls in approximately 3 seconds.
- The canary requested `priority`, and gateway usage recorded `priority`; the
  upstream completion itself reported `service_tier=default`. This proves the
  history compatibility fix, not that the upstream granted Fast on that attempt.
  Fast forwarding/policies have separate offline regression coverage.
- Public HTTPS homepage, local health, and admin SPA routes returned 200.
- CC Switch database provider records, settings, Codex config and model catalog
  matched the pre-upgrade snapshot; listeners remained on PIDs 37452/8292 at
  ports 15721/15722. No session files were rewritten.
- After the canary, the broad `codex_*` snapshot changed because it includes
  quota percentages and update timestamps. An independent restore of the
  cutover account table confirmed that fingerprint seeds, modes and identity
  flags were identical for all 58 OpenAI rows, including soft-deleted rows.
  Future post-traffic identity comparisons must exclude quota telemetry.
- All temporary rehearsal databases and restored private staging directories
  were removed after verification; production and local rollback backups remain.
