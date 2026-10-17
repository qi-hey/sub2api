# October 17 DNS and Grok Recovery

## Current Status

Production is `0.2.16-r79`, executable SHA256
`142109113e14c52721b84db6e2880ed8ab7768883bc968ac4aca0149e49fb9a2`.
The DNS repair is active. Following the user's explicit approval, the
additional local-route canary passed at 19:43 Asia/Shanghai. Earlier failed
verification and rollbacks are retained below rather than overwritten.

## DNS Incident

Times below use Asia/Shanghai on 2026-10-17.

- 06:35: r79 deployed; successful Grok and GPT usage followed.
- 08:46: VPS rebooted.
- 08:47: Tailscale selected openresolv mode, failed to read the OS resolver
  configuration, and reported no upstream resolvers and SERVFAIL.
- 17:49 onward: Grok and GPT requests failed resolving their hostnames via
  `fd7a:115c:a1e0::53`. CC Switch subsequently opened its circuit.
- 18:03: r78 restored; startup still logged DNS failures.
- 18:05: another task disabled Tailscale accept-dns and restored public
  servers in `/etc/resolv.conf`.
- The current task found the static file working but systemd-resolved still
  advertising the old Tailscale resolvers.

The `resolvconf` executable is a symlink to `resolvectl`; it rejects the
openresolv-style `-l` call. The exact origin of that system configuration
change was not established. No r79 source or deployment diff changes DNS.

## Applied DNS Repair

- Kept the already-persisted Tailscale `CorpDNS=false` preference.
- Preserved the working static `/etc/resolv.conf` byte-for-byte.
- Added `/etc/systemd/resolved.conf.d/90-sub2api-public-dns.conf`:
  primary servers 1.1.1.1 / 8.8.8.8; fallback 1.0.0.1 / 8.8.4.4.
- Restarted only systemd-resolved for this step. Tailscaled and Sub2API
  process IDs remained unchanged during the DNS change.
- Tested candidate DNS in a private mount namespace before application,
  then verified root, the sub2api OS user and systemd-resolved all resolve
  both OpenAI and Grok. TLS verification succeeded for both endpoints.
- A host reboot was not performed. Persistent configuration and a DNS
  service restart were tested; whole-host boot validation is not claimed.
- Reusable helper: `deploy/scripts/vps-public-dns-recovery.py`.

The configured NAS backup peer is offline in Tailscale and its TCP endpoint
was unreachable. No NAS, route, backup-target or Tailscale account settings
were changed. Existing timer status alone is not proof of backup delivery.

## Grok Validation

- Re-ran r79/r78 compatibility, fingerprint, Fast, Grok retry and passthrough
  tests, handler delegation tests and service vet.
- Private replay retains all 62 readable message/content strings across
  375 history items. New full-forward replay also verifies one bounded
  encrypted-reasoning retry, retained latest text/image, retained completed
  agent result, and immutable source request.
- One isolated Codex session was resumed GPT -> Grok -> GPT against a local
  mock. The history marker persisted in all three requests. GPT priority,
  Grok without priority, and GPT normal were verified. This CLI sequence used
  high/xhigh; max/ultra catalog entries were checked for preservation, not
  claimed as live-model reasoning tests.
- Copied-state rollback and unchanged production client files/processes
  passed. The 80 existing session files were neither removed nor truncated.
- A fresh VPS backup was restored into isolated databases. Candidate r79
  and restored r78 both passed health/site/game-asset checks. Database and
  config restoration matched; staging resources were cleaned up.

## Controlled Live Request

An initial verifier preflight incorrectly looked for an API key in the live
OAuth auth.json, failed before sending any HTTP request, and triggered a
binary rollback. The verifier now reads the existing provider bearer first.
No authentication or CC Switch configuration was changed.

At 19:21:21 r79 was installed again. Exactly one model request was sent through
the existing localhost 15722 route, CC Switch and the VPS:

- Model: Grok 4.7; reasoning: xhigh.
- HTTP 200; response.completed; response model grok-4.7-build.
- No tool calls; total elapsed 9534 ms.
- Usage record 186268: account 32090, 320 input tokens, 768 output tokens.
- Expected synthetic final-answer marker was not detected.

The old verifier did not retain the actual returned text, so the available
evidence cannot distinguish response parsing omissions from a model answer
mismatch. Do not label this as either a confirmed new Grok bug or a fully
passed old-history test. The guard restored r78 and kept the DNS fix.

Prepared `grok_canary_events.py` now captures visible synthetic output from
delta, text-done, item-done and terminal events, plus event types and usage.
It excludes encrypted reasoning. Seven offline parser regressions passed.
The original one-shot marker was kept intact. A second probe was not sent
until the user explicitly answered the request for an additional test.

Seven normal GPT-6 Astra requests succeeded after the final rollback at the
last check. No new DNS error was observed. One earlier 429 was an account
usage-limit response recovered with HTTP 200, separate from this incident.

## Authorized Additional Test

The user replied "test" after the explicit request for one additional
controlled request. The additional run has a separate one-shot marker and
report, preserving the first result.

- Candidate r79 was restored at 19:43:35 Asia/Shanghai, process PID 95197.
- Request traversed the unchanged local port 15722, CC Switch and VPS.
- Model Grok 4.7, xhigh, HTTP 200, text/event-stream.
- Response model grok-4.7-build, status completed; zero tool calls.
- Visible response was exactly `R79-GROK47-OK`; duration 2671 ms.
- Output included both reasoning and message items. The verifier observed
  eight text delta events, text-done, item-done and response.completed.
- Report: `backups/r79-recovery-20261017/canary-result-approved-extra.json`
  in the local project. No private user history was sent.
- After deployment, one normal GPT-6 Astra usage record and the Grok canary
  were present, with no new ops error at the final check.
- CC Switch remained healthy with zero consecutive failures. Provider
  configuration, selected model/effort/tier, model catalog, proxy PIDs and
  all 80 existing session files passed the unchanged-state check.
- The service remains on r79 after this successful test.

The first semantic mismatch cannot be retrospectively attributed to either
the parser or the model, because its actual answer was not retained. The
additional test demonstrates successful current behavior, not an explanation
of the missing earlier text. Opaque-only history without a readable completed
agent result remains outside the r79 recovery contract.

## Backup and Protected State

- DNS backup: `/opt/sub2api/backups/dns-before-r79-recovery-20261017T111224Z`.
- Fresh application backup:
  `/opt/sub2api/backups/r78-before-r79-20261017T111517Z`.
- Local copies:
  `D:/Codex/Codex-VPS-Sub2api/backups/r79-recovery-20261017`.
- Database SHA256:
  `981500324430f03f976e1a146d88514f7c773f27918c15aea5e76e57c90d7e1a`.
- Service archive SHA256:
  `9914881dfefd776c45ddbbf175fc0346e447fcc728cba8bc28aa90a400962215`.
- Current client model/effort/tier, catalog/order, 500000/490000 limits and
  ports 15721/15722 were preserved. No client restart or provider switch.
- Account fingerprints, mappings, priorities and group bindings were not
  rewritten. Rollback swaps only the binary and never restores broken DNS
  or an older live database over newer traffic.
