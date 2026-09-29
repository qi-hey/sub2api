## CC Switch / Sub2API production safety gate

These rules apply only to the `D:\Codex\Codex-VPS-Sub2api` project and its
Sub2API worktrees, CC Switch integration, Codex provider configuration, local
proxy, and production VPS.

1. Diagnose read-only first. If the user asks only for confirmation or
   investigation, do not edit files, restart processes, switch providers, clear
   caches, deploy, or send repeated live model probes.
2. Treat CC Switch as a multi-file live state system. Before an approved
   change, back up and record all of the following together:
   `cc-switch.db`, `settings.json`, Codex `config.toml`, the active user model
   catalog, the selected provider, relevant process IDs, and listeners on ports
   `15721` and `15722`. A database-only backup is not a complete rollback.
3. Never test an unverified CC Switch change against the live Codex setup.
   First reproduce it with copied state, an isolated `CODEX_HOME` or equivalent
   test directory, and a temporary non-production port. Validate that the
   original live files and processes remain unchanged.
4. The pre-deployment matrix must cover normal GPT requests, Grok requests,
   Fast/Priority behavior where supported, model switching in both directions,
   model catalog/order preservation, session-history preservation, local proxy
   takeover, and a full rollback rehearsal.
5. Grok request/tool-schema compatibility fixes belong in the Sub2API Grok
   outbound adapter and must be scoped to Grok. Do not modify CC Switch,
   provider routing, the GPT path, model ordering, or the user-selected model
   merely to repair a Grok schema error.
6. Apply only the smallest verified production change. Keep CC Switch running
   unless stopping it is explicitly required and the tested recovery sequence
   is ready. Do not change the Codex `base_url`, the `PROXY_MANAGED` placeholder,
   provider selection, or the roles of ports `15721`/`15722` without an explicit
   user request.
7. After offline tests pass, use at most one controlled live canary before
   normal traffic. Do not repeatedly send known-failing `/v1/responses`
   requests, because they can open the CC Switch circuit breaker and hide the
   original fault behind a 503 error.
8. Verify rollback before declaring success. Restoration must cover every file
   and the process startup order, then confirm the listener, provider, model,
   reasoning effort, Fast setting, session history, and upstream route.
9. Never alter the user's current model, reasoning effort, service tier,
   fingerprints, account mappings, allowlists, provider priorities, or session
   records unless the current request explicitly requires that exact change.
10. For Sub2API upgrades, build and test locally first, preserve downstream
    customizations, compare migrations and configuration effects, create a
    complete VPS backup, and deploy only after the isolated validation passes.
