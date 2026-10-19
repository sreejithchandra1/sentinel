package alerter

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/sentinel-monitoring/sentinel/internal/safehost"
)

func outboundHTTPClient(timeout time.Duration) *http.Client {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS13}
	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
		DialContext:     safehost.ControlDialContext,
	}
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
		transport.TLSClientConfig = tlsCfg
		transport.DialContext = safehost.ControlDialContext
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Do not follow redirects — a 30x to a private IP would bypass hostname checks on the original URL.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
