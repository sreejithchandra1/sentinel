package alerter

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"
)

func TestOutboundHTTPClientRequiresTLS13(t *testing.T) {
	client := outboundHTTPClient(5 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type %T", client.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("missing TLSClientConfig")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion=%d want TLS 1.3", transport.TLSClientConfig.MinVersion)
	}
}
