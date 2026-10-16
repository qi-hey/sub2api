# v0.2.16-r79 Validation

## Failure and Scope

Production error 11378 at 2026-10-16 22:08:27 UTC was the r78 local input
guard rejecting an opaque encrypted `agent_message`. It was not a new xAI
schema rejection or a CC Switch configuration failure.

The private failing history, reconstructed from its most recent compaction,
contained 375 items. One historical agent's intermediate message contained
opaque encrypted content; the same agent's following `FINAL_ANSWER` was
readable. The request also retained readable conversation context.

r79 marks only that unavailable intermediate payload, retaining its
envelope, readable siblings and the later final answer. Matching requires
the same author/recipient and a complete Codex envelope, before a later
user turn. Task reuse and malformed boundaries prevent a match. This does
not decrypt or reconstruct encrypted text, and is not a general solution
for encrypted-only history without a readable completed result.

Only Grok outbound files and the release version change. Unrecoverable
input errors now use `param: input`. No database migration, account change,
GPT HTTP/WS change, fingerprint change or CC Switch edit is included.

## Local Verification

- Complete backend `go test -tags=unit ./...`: passed.
- Complete backend `go vet -tags=unit ./...`: passed before the final
  malformed-boundary guard; focused tests were rerun after that addition.
- New tests cover completed agent history, multiple/interleaved agents,
  CRLF envelopes, idempotence, immutable original requests, JSON/SSE
  forwarding and correct input-error attribution.
- Negative tests cover absent/earlier/empty/encrypted final answers,
  different senders or recipients, task reuse, active encrypted tasks,
  missing/mismatched envelopes, malformed boundaries and ordinary messages.
- R78 and audit regression tests were rerun together with r79.
- Private 24,221,653-byte replay: all 62 readable message/content strings
  retained, one opaque agent part explicitly marked, source bytes unchanged.
  Private contents remain outside Git and are never sent to the live canary.
- OpenAI shared WS adapter retains the original encrypted payload. Native
  Grok reasoning replay remains unchanged. Existing GPT, Fast, fingerprint
  and model-switching tests pass in the full backend suite.

## Recovery

- Fresh VPS backup:
  `/opt/sub2api/backups/r78-before-r79-20261016T222203Z`.
- Database dump SHA256:
  `955e7472d9d0d812ad15416f43864c997a08a720ab6a6e4469305279a186035e`.
- Service archive SHA256:
  `f6ac3f761c933162575821354c597efa369c3542e3ed9427f987dc4069439e0f`.
- Original r78 executable SHA256:
  `afcde86c48b42026fa3f2aeebdd436e33f132c324d1193a5768eb62a2ae8c78a`.
- Local backup/validation directory:
  `D:/Codex/Codex-VPS-Sub2api/backups/r79-validation`.
- A read-only client-state guard records CC Switch providers/settings,
  Codex configuration/catalog, proxy listeners and existing session files.
  This release does not require a client restart.

## Deployment Gate

Build locally with embedded frontend. Restore the new backup into isolated
databases, run candidate and restored r78 binaries on temporary ports with
external network access denied, and verify restored config hashes and site
assets. Production cutover checks protected fingerprints, mappings, groups
and configuration hashes; failures restore the old executable.

After cutover, permit one synthetic encrypted-history Grok 4.7 canary.
Require a completed response containing the retained final-answer marker;
otherwise restore r78. Use normal traffic records to verify GPT rather than
repeated paid probes. Record the actual results below when available.
