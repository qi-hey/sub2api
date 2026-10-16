# v0.2.16-r78 Validation

## Scope

Repair the nine locally reproduced Grok compatibility cases from the October
17 audit. The production trigger was error 11375: xAI rejected the nested
`input[57].content[1]` encrypted-content carrier.

Only Grok outbound behavior changes. The shared OpenAI WS adapter retains
its prior branch. Fingerprint implementations, seeds, group bindings,
provider priorities, CC Switch routes and sessions must remain unchanged.

## Verification

- Complete backend `go test -tags=unit ./...`: passed.
- Complete backend `go vet -tags=unit ./...`: passed.
- New audit regressions and Forward-level JSON/SSE delegation tests: passed.
- Frontend: 390 files, 3006 tests passed; production build passed.
- New tests cover Grok 4.7/xhigh and preservation of native reasoning state,
  explicit failure for opaque encrypted message bodies, and unchanged
  OpenAI WS behavior including priority/reasoning settings.
- CC Switch isolated CLI sequence GPT priority -> Grok 4.7 xhigh -> GPT
  normal completed against loopback fixtures. All copied files and the
  database were restored in the rollback rehearsal. No paid model requests
  were used for local configuration testing.
- Candidate Linux amd64 binary was built locally with embedded frontend.
  SHA256: `afcde86c48b42026fa3f2aeebdd436e33f132c324d1193a5768eb62a2ae8c78a`.
- Final focused regression selection: 118 top-level tests passed. Nested union
  equivalence independently checked across 36 argument combinations.

## Recovery

- Full VPS backup: `/opt/sub2api/backups/r77-before-r78-20261016T194921Z`.
- Database and service archive also downloaded into project-local `backups`.
- Original executable SHA256:
  `bdbe2e9b9697641d6ec8c9221402128acf773214df61bbfa99e150b8c414dba9`.
- CC Switch complete snapshot:
  `C:/Users/Administrator/.cc-switch/backups/before-grok47-r78-20261017-0356`.
- Account mapping migration preserves source aliases, redirects only old
  4.5 targets and adds `grok-4.7 -> grok-4.7`. OAuth credentials, status,
  concurrency, groups and priorities are not rewritten.
- A historical database trigger forced 4.5 targets after credentials updates.
  The rehearsal exposed this before production. The deployment SQL replaces
  that function with 4.7 missing-default insertion, while preserving explicit
  overrides. Rollback restores both the original function and original mappings.
- The previous Grok 4.6 model remains accepted server-side for existing
  sessions. Only the local picker entry is replaced.

## Deployment Results

- Deployed on 2026-10-17 Asia/Shanghai as `0.2.16-r78`; the running process
  executable matches the candidate SHA256.
- Cutover backup: `/opt/sub2api/backups/r78-cutover-20261016T201231Z`.
  Fresh dump SHA256:
  `71ce40070ee36d64ceddfa427958fed0ff9b63a58aa8b3dd1002d1bc10436865`.
- All 1149 non-deleted Grok accounts now have the 4.7 identity; zero old
  4.5 targets remain, and zero accounts have concurrency above two.
- Restored-database tests verified mapping idempotence, default insertion,
  credentials-refresh persistence, and restoration of both mappings and the
  previous trigger function. Both r78 and restored r77 passed health, homepage,
  admin SPA, game image and audio checks on isolated ports.
- One initial diagnostic fixture was rejected by HTTP ingress because its
  named tool result lacked the required delegation envelope. Error 11377 has
  no account ID, upstream status or upstream latency; no xAI call occurred.
  Added a handler-level regression for the corrected real Codex envelope.
- One upstream canary with nested message content, a delegation envelope and
  a nested union tool schema returned HTTP 200, `response.completed`, and
  `R78-GROK47-OK`, with zero new tool calls in 1493 ms. The upstream response
  model was `grok-4.7-build`.
- Post-deployment usage records showed 41 GPT-6.1 Sol successes, 16 GPT-6 Astra
  successes and the Grok 4.7 canary. No post-deployment Grok upstream error
  was recorded at the final check.
- Fingerprint identity, group settings and account/key group hashes match
  the cutover baseline. No persistent GPT configuration was changed.
- The shared CC Switch model catalog replaces Grok 4.6 at the same position.
  The then-selected Grok 4.6 live config becomes Grok 4.7 with xhigh; saved GPT
  provider defaults, all GPT metadata, proxy listeners/processes and the
  500000/490000 limits are preserved. The database/settings file are unchanged.
  All 101 session files present in the snapshot remained; one new session
  appeared independently during work. No session history file was edited.
