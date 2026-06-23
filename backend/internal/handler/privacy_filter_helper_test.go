package handler

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestApplyPrivacyFilterToRequestBodyRedactsEmail(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"contact me at test@example.com"}]}`)

	redacted := applyPrivacyFilterToRequestBody(zaptest.NewLogger(t), "openai_chat", "gpt-4", body)

	if bytes.Equal(redacted, body) {
		t.Fatal("expected body to be changed")
	}
	if strings.Contains(string(redacted), "test@example.com") {
		t.Fatalf("email was not redacted: %s", string(redacted))
	}
}

func TestApplyPrivacyFilterToRequestBodyLeavesNoHitUnchanged(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello world"}]}`)

	redacted := applyPrivacyFilterToRequestBody(zaptest.NewLogger(t), "openai_chat", "gpt-4", body)

	if !bytes.Equal(redacted, body) {
		t.Fatalf("expected body to be unchanged, got %s", string(redacted))
	}
}
