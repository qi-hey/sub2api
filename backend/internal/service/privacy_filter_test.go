package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestRedactPrivacyRequestBodyRedactsChatMessageContent(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"my email is test@example.com"}]}`)

	redacted, changed, err := RedactPrivacyRequestBody(body)

	if err != nil {
		t.Fatalf("RedactPrivacyRequestBody() error = %v", err)
	}
	if !changed {
		t.Fatal("expected body to be redacted")
	}
	if strings.Contains(string(redacted), "test@example.com") {
		t.Fatalf("email was not redacted: %s", string(redacted))
	}
	if !gjson.ValidBytes(redacted) {
		t.Fatalf("redacted body is not valid JSON: %s", string(redacted))
	}
}

func TestRedactPrivacyRequestBodyRedactsMultipartText(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":[{"type":"text","text":"my phone is 13800138000"}]}]}`)

	redacted, changed, err := RedactPrivacyRequestBody(body)

	if err != nil {
		t.Fatalf("RedactPrivacyRequestBody() error = %v", err)
	}
	if !changed {
		t.Fatal("expected body to be redacted")
	}
	if strings.Contains(string(redacted), "13800138000") {
		t.Fatalf("phone number was not redacted: %s", string(redacted))
	}
}

func TestRedactPrivacyRequestBodyRedactsResponsesStringInput(t *testing.T) {
	body := []byte(`{"model":"gpt-4","input":"my api key is sk-proj-abcdefghijklmnopqrstuvwxyz123456"}`)

	redacted, changed, err := RedactPrivacyRequestBody(body)

	if err != nil {
		t.Fatalf("RedactPrivacyRequestBody() error = %v", err)
	}
	if !changed {
		t.Fatal("expected body to be redacted")
	}
	if strings.Contains(string(redacted), "sk-proj-abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("secret was not redacted: %s", string(redacted))
	}
}

func TestRedactPrivacyRequestBodyReturnsOriginalForNoHit(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello world"}]}`)

	redacted, changed, err := RedactPrivacyRequestBody(body)

	if err != nil {
		t.Fatalf("RedactPrivacyRequestBody() error = %v", err)
	}
	if changed {
		t.Fatalf("expected no redaction, got: %s", string(redacted))
	}
	if !bytes.Equal(redacted, body) {
		t.Fatalf("unchanged body mismatch: got %s want %s", string(redacted), string(body))
	}
}

func TestRedactPrivacyRequestBodyReturnsOriginalForInvalidJSON(t *testing.T) {
	body := []byte(`not json with email test@example.com`)

	redacted, changed, err := RedactPrivacyRequestBody(body)

	if err != nil {
		t.Fatalf("RedactPrivacyRequestBody() error = %v", err)
	}
	if changed {
		t.Fatalf("expected invalid JSON to pass through, got: %s", string(redacted))
	}
	if !bytes.Equal(redacted, body) {
		t.Fatalf("invalid JSON body mismatch: got %s want %s", string(redacted), string(body))
	}
}
