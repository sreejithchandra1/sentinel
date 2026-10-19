package alerter

import (
	"os"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/safehost"
)

func TestMain(m *testing.M) {
	// Unit tests dial loopback httptest servers for Slack/webhooks.
	safehost.AllowNonPublicForPackageTests()
	os.Exit(m.Run())
}
