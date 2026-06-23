package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

func applyPrivacyFilterToRequestBody(reqLog *zap.Logger, protocol, model string, body []byte) []byte {
	redacted, changed, err := service.RedactPrivacyRequestBody(body)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("privacy_filter.redact_failed",
				zap.String("protocol", protocol),
				zap.String("model", model),
				zap.Int("body_bytes", len(body)),
				zap.Error(err),
			)
		}
		return body
	}
	if !changed {
		return body
	}
	if reqLog != nil {
		reqLog.Info("privacy_filter.redacted",
			zap.String("protocol", protocol),
			zap.String("model", model),
			zap.Int("body_bytes", len(body)),
			zap.Int("redacted_body_bytes", len(redacted)),
		)
	}
	return redacted
}
