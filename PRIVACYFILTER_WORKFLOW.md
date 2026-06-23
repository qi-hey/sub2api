# Privacyfilter Update Workflow

This repository keeps the Sub2API upstream source plus a small privacyfilter patch.

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
