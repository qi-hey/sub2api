# Privacyfilter Update Workflow

This repository keeps the Sub2API upstream source plus downstream customizations
that must survive every upstream update.

## Required downstream customizations

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

Branches:

- `upstream-clean`: official Sub2API source without local changes.
- `privacyfilter-v137`: the original privacyfilter patch extracted from the VPS build.
- `main`: current deployable branch.
- `deploy`: alias branch for the current deployable branch.

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
