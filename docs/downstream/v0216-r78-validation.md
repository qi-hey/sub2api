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
- The previous Grok 4.6 model remains accepted server-side for existing
  sessions. Only the local picker entry is replaced.

Deployment and live canary results must be recorded after verification.
