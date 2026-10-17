import json
import unittest

from grok_canary_events import read_canary_events


class CanaryEventsTest(unittest.TestCase):
    def parse(self, events):
        return read_canary_events(
            [b"data: " + json.dumps(event).encode() + b"\n" for event in events], "FIXTURE_OK")

    def completed(self, output=None):
        return {"type": "response.completed", "response": {
            "model": "grok-4.7", "status": "completed", "output": output or []}}

    def message(self, text):
        return {"type": "message", "content": [{"type": "output_text", "text": text}]}

    def test_delta_stream(self):
        got = self.parse([{"type": "response.output_text.delta", "delta": "FIXTURE_"},
                          {"type": "response.output_text.delta", "delta": "OK"}, self.completed()])
        self.assertTrue(got["ok"])

    def test_done_only_stream(self):
        got = self.parse([{"type": "response.output_text.done", "text": "FIXTURE_OK"}, self.completed()])
        self.assertTrue(got["ok"])

    def test_item_done_only_stream(self):
        got = self.parse([{"type": "response.output_item.done", "item": self.message("FIXTURE_OK")},
                          self.completed()])
        self.assertTrue(got["ok"])

    def test_terminal_text_after_partial_delta(self):
        got = self.parse([{"type": "response.output_text.delta", "delta": "FIX"},
                          self.completed([self.message("FIXTURE_OK")])])
        self.assertTrue(got["ok"])

    def test_completed_without_visible_answer_fails(self):
        got = self.parse([self.completed([{"type": "reasoning", "encrypted_content": "secret-fixture"}])])
        self.assertFalse(got["ok"])
        self.assertNotIn("secret-fixture", json.dumps(got))

    def test_wrong_answer_is_recorded_but_fails(self):
        got = self.parse([self.completed([self.message("different answer")])])
        self.assertFalse(got["ok"])
        self.assertEqual(["different answer"], got["visible_text_samples"])

    def test_failure_cannot_pass_with_marker(self):
        got = self.parse([{"type": "response.output_text.delta", "delta": "FIXTURE_OK"},
                          {"type": "response.failed"}])
        self.assertFalse(got["ok"])


if __name__ == "__main__":
    unittest.main()
