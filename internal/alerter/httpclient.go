package alerter

import (
	"crypto/tls"
	"net/http"
	"time"
)

func outboundHTTPClient(timeout time.Duration) *http.Client {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS13}
	transport := &http.Transport{TLSClientConfig: tlsCfg}
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
		transport.TLSClientConfig = tlsCfg
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
