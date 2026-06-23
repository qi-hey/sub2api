package service

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"privacyfilter/filter"
)

var (
	privacyFilterOnce sync.Once
	privacyFilterInst *filter.Filter
	privacyFilterErr  error
)

//go:embed privacy_filter_rules/gitleaks.toml
var embeddedPrivacyFilterGitleaks []byte

func RedactPrivacyRequestBody(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	f, err := defaultPrivacyFilter()
	if err != nil {
		return body, false, err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, false, nil
	}

	field := "messages"
	items, ok := payload[field]
	if !ok {
		field = "input"
		items, ok = payload[field]
	}
	if !ok {
		return body, false, nil
	}

	changed := false
	switch v := items.(type) {
	case string:
		changed = redactPrivacyText(f, &v)
		if changed {
			payload[field] = v
		}
	case []any:
		changed = redactPrivacyContentItems(f, v)
		if changed {
			payload[field] = v
		}
	default:
		return body, false, nil
	}
	if !changed {
		return body, false, nil
	}

	out, err := json.Marshal(payload)
	if err != nil {
		return body, false, fmt.Errorf("marshal privacy-filtered request: %w", err)
	}
	return out, true, nil
}

func defaultPrivacyFilter() (*filter.Filter, error) {
	privacyFilterOnce.Do(func() {
		privacyFilterInst, privacyFilterErr = newEmbeddedPrivacyFilter()
	})
	return privacyFilterInst, privacyFilterErr
}

func newEmbeddedPrivacyFilter() (*filter.Filter, error) {
	if len(embeddedPrivacyFilterGitleaks) == 0 {
		return filter.New("")
	}

	tmp, err := os.CreateTemp("", "sub2api-privacyfilter-gitleaks-*.toml")
	if err != nil {
		return nil, fmt.Errorf("create privacy filter rules file: %w", err)
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(embeddedPrivacyFilterGitleaks); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write privacy filter rules file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close privacy filter rules file: %w", err)
	}

	f, err := filter.New(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("create privacy filter: %w", err)
	}
	return f, nil
}

func redactPrivacyContentItems(f *filter.Filter, items []any) bool {
	changed := false
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, ok := itemMap["content"]
		if !ok {
			continue
		}
		if redactPrivacyContent(f, &content) {
			itemMap["content"] = content
			changed = true
		}
	}
	return changed
}

func redactPrivacyContent(f *filter.Filter, content *any) bool {
	changed := false
	switch v := (*content).(type) {
	case string:
		if redactPrivacyText(f, &v) {
			*content = v
			changed = true
		}
	case []any:
		for j, part := range v {
			partMap, ok := part.(map[string]any)
			if !ok {
				continue
			}
			text, ok := partMap["text"].(string)
			if !ok {
				continue
			}
			if redactPrivacyText(f, &text) {
				partMap["text"] = text
				v[j] = partMap
				changed = true
			}
		}
	}
	return changed
}

func redactPrivacyText(f *filter.Filter, text *string) bool {
	if f == nil || text == nil || *text == "" {
		return false
	}
	result := f.Redact(*text)
	if !result.Hit {
		return false
	}
	*text = result.Redacted
	return true
}
