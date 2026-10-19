package alerter

import (
	"fmt"
	"strings"

	"github.com/sentinel-monitoring/sentinel/internal/safehost"
)

func validateOutboundWebhookURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("webhook URL is required")
	}
	if err := safehost.ValidateHTTPURL(raw); err != nil {
		return fmt.Errorf("webhook URL is not allowed")
	}
	return nil
}
