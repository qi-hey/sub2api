"""Pin public DNS for systemd-resolved without changing working resolv.conf.

Prepare and test in a private mount namespace before applying. Tailscale must
already have accept-dns disabled. No application, Tailscale or host restart.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time

os.umask(0o077)
INCOMING = Path("/opt/sub2api/incoming")
MANIFEST = INCOMING / "r79-dns-recovery.json"
RESOLV = Path("/etc/resolv.conf")
DROPIN = Path("/etc/systemd/resolved.conf.d/90-sub2api-public-dns.conf")
CONFIG = "[Resolve]\nDNS=1.1.1.1 8.8.8.8\nFallbackDNS=1.0.0.1 8.8.4.4\n"
PUBLIC = "nameserver 1.1.1.1\nnameserver 8.8.8.8\noptions timeout:2 attempts:2\n"
HOSTS = ("chatgpt.com", "cli-chat-proxy.grok.com")


def command(args, timeout=15):
    result = subprocess.run(args, capture_output=True, text=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError("diagnostic command failed: " + args[0])
    return result.stdout


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def service_pid(name):
    return command(["systemctl", "show", name, "-p", "MainPID", "--value"]).strip()


def tailscale_prefs():
    prefs = json.loads(command(["tailscale", "debug", "prefs"]))
    return {key: prefs.get(key) for key in ("CorpDNS", "WantRunning", "RouteAll")}


def lookup_checks(include_resolved=False):
    result = []
    for host in HOSTS:
        for user in (None, "sub2api"):
            prefix = ["runuser", "-u", user, "--"] if user else []
            data = command(prefix + ["getent", "ahostsv4", host])
            assert data.strip(), "empty DNS result"
            result.append({"host": host, "user": user or "root", "resolved": True})
        if include_resolved:
            command(["resolvectl", "query", "--type=A", host])
            result.append({"host": host, "systemd_resolved": True})
    return result


def prepare():
    assert not MANIFEST.exists(), "recovery manifest already exists"
    prefs = tailscale_prefs()
    assert prefs["CorpDNS"] is False and prefs["WantRunning"] is True
    assert not RESOLV.is_symlink(), "expected the currently working static resolver file"
    text = RESOLV.read_text()
    assert "nameserver 1.1.1.1" in text and "nameserver 8.8.8.8" in text
    assert "nameserver 100.100.100.100" not in text and "fd7a:" not in text
    root = Path("/opt/sub2api/backups") / (
        "dns-before-r79-recovery-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()))
    root.mkdir(mode=0o700)
    shutil.copy2(RESOLV, root / "resolv.conf")
    if DROPIN.exists():
        shutil.copy2(DROPIN, root / "resolved-dropin.conf")
    (root / "candidate-resolv.conf").write_text(PUBLIC)
    os.chmod(root / "candidate-resolv.conf", 0o644)
    (root / "candidate-resolved.conf").write_text(CONFIG)
    original = sha(RESOLV)
    before = {name: service_pid(name) for name in ("sub2api", "tailscaled")}
    checks = json.loads(command([
        "unshare", "--mount", "--propagation", "private", sys.executable,
        str(Path(__file__).resolve()), "namespace-probe", str(root / "candidate-resolv.conf"),
    ], timeout=40))
    assert sha(RESOLV) == original
    assert {name: service_pid(name) for name in before} == before
    report = {"backup": str(root), "resolv_sha256": original,
              "dropin_existed": DROPIN.exists(),
              "old_dropin_sha256": sha(DROPIN) if DROPIN.exists() else None,
              "service_pids": before, "tailscale_preferences": prefs,
              "isolated_lookup_checks": checks, "host_rebooted": False,
              "production_resolv_unchanged": True}
    MANIFEST.write_text(json.dumps(report, indent=2))
    print(json.dumps(report), flush=True)


def verify(check_application_pid=True):
    baseline = json.loads(MANIFEST.read_text())
    assert sha(RESOLV) == baseline["resolv_sha256"]
    assert DROPIN.read_text() == CONFIG
    assert tailscale_prefs() == baseline["tailscale_preferences"]
    assert service_pid("tailscaled") == baseline["service_pids"]["tailscaled"]
    if check_application_pid:
        assert service_pid("sub2api") == baseline["service_pids"]["sub2api"]
    result = lookup_checks(include_resolved=True)
    resolver = command(["resolvectl", "dns"])
    assert "1.1.1.1" in resolver and "8.8.8.8" in resolver
    assert "100.100.100.100" not in resolver and "fd7a:115c:a1e0::53" not in resolver
    report = {"checks": result, "system_resolver_uses_public_dns": True,
              "tailscale_pid_unchanged": True,
              "application_pid_checked": check_application_pid,
              "static_resolv_unchanged": True, "host_rebooted": False,
              "persistent_dropin": str(DROPIN)}
    (INCOMING / "r79-dns-verification.json").write_text(json.dumps(report, indent=2))
    print(json.dumps(report), flush=True)


def apply():
    baseline = json.loads(MANIFEST.read_text())
    assert sha(RESOLV) == baseline["resolv_sha256"]
    assert DROPIN.exists() == baseline["dropin_existed"]
    if DROPIN.exists():
        assert sha(DROPIN) == baseline["old_dropin_sha256"]
    assert tailscale_prefs() == baseline["tailscale_preferences"]
    DROPIN.parent.mkdir(exist_ok=True)
    try:
        temporary = DROPIN.with_suffix(".tmp")
        temporary.write_text(CONFIG)
        os.chmod(temporary, 0o644)
        os.replace(temporary, DROPIN)
        command(["systemctl", "restart", "systemd-resolved"], timeout=30)
        verify()
    except BaseException:
        if baseline["dropin_existed"]:
            shutil.copy2(Path(baseline["backup"]) / "resolved-dropin.conf", DROPIN)
        else:
            DROPIN.unlink(missing_ok=True)
        command(["systemctl", "restart", "systemd-resolved"], timeout=30)
        raise


if __name__ == "__main__":
    if sys.argv[1] == "namespace-probe":
        command(["mount", "--bind", sys.argv[2], str(RESOLV)])
        print(json.dumps(lookup_checks()))
    elif sys.argv[1] == "verify-network":
        verify(check_application_pid=False)
    else:
        {"prepare": prepare, "apply": apply, "verify": verify}[sys.argv[1]]()
