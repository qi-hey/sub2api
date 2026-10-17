"""Read visible synthetic-canary output without retaining encrypted reasoning."""
import collections
import json


def read_canary_events(lines, marker):
    deltas, snapshots = [], []
    kinds = collections.Counter()
    report = {"completed": False, "tool_calls": 0}

    def message_text(item):
        if item.get("type") != "message":
            return ""
        return "".join(part.get("text", "") for part in item.get("content", [])
                       if isinstance(part, dict) and isinstance(part.get("text"), str))

    for line in lines:
        if not line.startswith(b"data:"):
            continue
        try:
            event = json.loads(line[5:].strip())
        except ValueError:
            continue
        if not isinstance(event, dict):
            continue
        kind = event.get("type", "")
        kinds[kind] += 1
        if kind == "response.output_text.delta" and isinstance(event.get("delta"), str):
            deltas.append(event["delta"])
        if kind == "response.output_text.done" and isinstance(event.get("text"), str):
            snapshots.append(event["text"])
        if kind == "response.output_item.done":
            snapshots.append(message_text(event.get("item", {})))
        if kind == "response.output_item.added" and event.get("item", {}).get("type") in {
            "function_call", "custom_tool_call", "web_search_call", "x_search_call"
        }:
            report["tool_calls"] += 1
        if kind in ("error", "response.failed", "response.incomplete"):
            report["terminal_event"] = kind
            break
        if kind == "response.completed":
            report["completed"] = True
            result = event.get("response", {})
            report["response_model"] = result.get("model")
            report["response_status"] = result.get("status")
            report["output_types"] = [item.get("type") for item in result.get("output", [])]
            report["usage"] = result.get("usage", {})
            snapshots.extend(message_text(item) for item in result.get("output", []))
            break
    candidates = ["".join(deltas)] + snapshots
    report["marker_preserved"] = any(marker in text for text in candidates)
    report["visible_text_samples"] = list(dict.fromkeys(text[:4000] for text in candidates if text))
    report["event_types"] = dict(kinds)
    report["ok"] = report["completed"] and report["tool_calls"] == 0 and report["marker_preserved"]
    return report
