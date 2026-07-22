#!/usr/bin/env python3

import argparse
import concurrent.futures
import fcntl
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter
from pathlib import Path


def load_env(path):
    values = {}
    with open(path, encoding="utf-8") as env_file:
        for raw_line in env_file:
            line = raw_line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            values[key] = value.strip().strip('"').strip("'")
    return values


class APIClient:
    def __init__(self, base_url, email, password, timeout):
        self.base_url = base_url.rstrip("/")
        self.email = email
        self.password = password
        self.timeout = timeout
        self.token = None

    def login(self):
        status, response = self.request(
            "POST",
            "/auth/login",
            payload={
                "email": self.email,
                "password": self.password,
                "turnstile_token": "",
            },
            authenticate=False,
        )
        if status != 200 or response.get("code") != 0:
            raise RuntimeError(f"admin login failed (HTTP {status})")
        self.token = response["data"]["access_token"]

    def request(self, method, path, payload=None, authenticate=True, timeout=None):
        body = None if payload is None else json.dumps(payload).encode()
        headers = {"Accept": "application/json"}
        if body is not None:
            headers["Content-Type"] = "application/json"
        if authenticate:
            if not self.token:
                raise RuntimeError("API client is not authenticated")
            headers["Authorization"] = "Bearer " + self.token

        request = urllib.request.Request(
            self.base_url + path,
            data=body,
            headers=headers,
            method=method,
        )
        try:
            with urllib.request.urlopen(request, timeout=timeout or self.timeout) as response:
                raw_body = response.read()
                data = json.loads(raw_body) if raw_body else {}
                return response.status, data
        except urllib.error.HTTPError as error:
            raw_body = error.read()
            try:
                data = json.loads(raw_body) if raw_body else {}
            except (TypeError, ValueError):
                data = {"message": "non-JSON error response"}
            return error.code, data
        except Exception as error:
            return 0, {"message": f"{type(error).__name__}: {error}"}

    def list_grok_accounts(self):
        accounts = []
        page = 1
        while True:
            query = urllib.parse.urlencode(
                {
                    "page": page,
                    "page_size": 1000,
                    "platform": "grok",
                    "sort_by": "id",
                    "sort_order": "asc",
                    "lite": "true",
                }
            )
            status, response = self.request("GET", "/admin/accounts?" + query)
            if status != 200 or response.get("code") != 0:
                raise RuntimeError(f"account list failed on page {page} (HTTP {status})")
            data = response["data"]
            accounts.extend(data["items"])
            if page >= data["pages"]:
                return accounts
            page += 1

    def account_schedulable(self, account_id):
        status, response = self.request("GET", f"/admin/accounts/{account_id}", timeout=15)
        if status != 200 or response.get("code") != 0:
            return None
        return (response.get("data") or {}).get("schedulable")

    def probe(self, account_id):
        started_at = time.monotonic()
        status, response = self.request(
            "GET",
            f"/admin/grok/accounts/{account_id}/quota?probe=active",
        )
        elapsed = round(time.monotonic() - started_at, 3)
        payload = (response.get("data") or {}) if isinstance(response, dict) else {}
        probe_status = payload.get("status_code")
        reason = response.get("reason", "") if isinstance(response, dict) else ""

        if status == 200 and probe_status == 429:
            category = "rate_limited"
        elif status == 429:
            category = "rate_limited"
        elif status == 200:
            category = "ok"
        elif status >= 400:
            schedulable = self.account_schedulable(account_id)
            if schedulable is False:
                category = "unavailable_disabled"
            elif status >= 500:
                category = "upstream_or_server_error"
            elif status == 401:
                category = "auth_error"
            else:
                category = "other_error"
        elif status == 0:
            category = "timeout_or_network"
        else:
            category = "other_error"

        return {
            "account_id": account_id,
            "category": category,
            "http_status": status,
            "probe_status": probe_status,
            "reason": reason,
            "elapsed_seconds": elapsed,
        }


def emit(payload):
    print(json.dumps(payload, ensure_ascii=False), flush=True)


def summarize_account_states(accounts):
    counts = Counter((item.get("status"), bool(item.get("schedulable"))) for item in accounts)
    return {
        f"{status}_schedulable_{str(schedulable).lower()}": count
        for (status, schedulable), count in counts.items()
    }


def parse_args():
    parser = argparse.ArgumentParser(description="Probe all schedulable Grok accounts safely.")
    parser.add_argument("--env-file", default="/etc/sub2api/sub2api.env")
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/api/v1")
    parser.add_argument("--workers", type=int, default=3)
    parser.add_argument("--timeout", type=int, default=35)
    parser.add_argument("--state-dir", default="/opt/sub2api/probe-reports")
    return parser.parse_args()


def main():
    args = parse_args()
    if args.workers < 1 or args.workers > 10:
        raise SystemExit("workers must be between 1 and 10")

    state_dir = Path(args.state_dir)
    state_dir.mkdir(parents=True, exist_ok=True)
    lock_path = state_dir / "grok-inventory-probe.lock"
    lock_file = open(lock_path, "w", encoding="utf-8")
    try:
        fcntl.flock(lock_file, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        raise SystemExit("another Grok inventory probe is already running")
    lock_file.write(str(os.getpid()))
    lock_file.flush()

    env = load_env(args.env_file)
    client = APIClient(
        args.base_url,
        env["ADMIN_EMAIL"],
        env["ADMIN_PASSWORD"],
        args.timeout,
    )
    client.login()

    before = client.list_grok_accounts()
    target_ids = [
        int(account["id"])
        for account in before
        if account.get("status") == "active" and account.get("schedulable") is True
    ]
    timestamp = time.strftime("%Y%m%d-%H%M%S", time.gmtime())
    details_path = state_dir / f"grok-inventory-probe-{timestamp}.jsonl"
    summary_path = state_dir / f"grok-inventory-probe-{timestamp}-summary.json"

    emit(
        {
            "event": "start",
            "all_grok": len(before),
            "targets": len(target_ids),
            "already_excluded": len(before) - len(target_ids),
            "workers": args.workers,
            "before": summarize_account_states(before),
            "details_path": str(details_path),
        }
    )

    counts = Counter()
    samples = {}
    started_at = time.monotonic()
    with open(details_path, "a", encoding="utf-8") as details_file:
        with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
            futures = [pool.submit(client.probe, account_id) for account_id in target_ids]
            for completed, future in enumerate(concurrent.futures.as_completed(futures), 1):
                result = future.result()
                category = result["category"]
                counts[category] += 1
                details_file.write(json.dumps(result, ensure_ascii=False) + "\n")
                details_file.flush()
                if category != "ok" and len(samples.setdefault(category, [])) < 20:
                    samples[category].append(result)

                if completed % 50 == 0 or completed == len(target_ids):
                    elapsed = time.monotonic() - started_at
                    rate = completed / elapsed if elapsed else 0
                    eta = (len(target_ids) - completed) / rate if rate else 0
                    emit(
                        {
                            "event": "progress",
                            "completed": completed,
                            "total": len(target_ids),
                            "ok": counts["ok"],
                            "unavailable_disabled": counts["unavailable_disabled"],
                            "rate_limited": counts["rate_limited"],
                            "errors": sum(
                                count
                                for key, count in counts.items()
                                if key not in ("ok", "unavailable_disabled", "rate_limited")
                            ),
                            "eta_seconds": round(eta),
                        }
                    )

    after = client.list_grok_accounts()
    summary = {
        "event": "complete",
        "duration_seconds": round(time.monotonic() - started_at, 1),
        "tested": len(target_ids),
        "counts": dict(counts),
        "before": summarize_account_states(before),
        "after": summarize_account_states(after),
        "samples": samples,
        "details_path": str(details_path),
    }
    with open(summary_path, "w", encoding="utf-8") as summary_file:
        json.dump(summary, summary_file, ensure_ascii=False, indent=2)
        summary_file.write("\n")
    summary["summary_path"] = str(summary_path)
    emit(summary)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        emit({"event": "fatal", "error": f"{type(error).__name__}: {error}"})
        sys.exit(1)
