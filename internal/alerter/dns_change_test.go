package alerter

import "testing"

const sampleDNSMsg = "A record changed from 147.79.69.222, 93.127.173.8 to 147.79.69.29, 91.108.106.189; AAAA record changed from 2a02:4780:1f:7f92:c347:cf3:e1b1:8f20, 2a02:4780:20:c74f:e17a:3e7f:2211:d536 to 2a02:4780:1f:4ae:49b1:4f3b:b23f:aace, 2a02:4780:3c:c2d3:d307:517:107d:de12"

func TestParseDNSChangeMessage(t *testing.T) {
	got := ParseDNSChangeMessage(sampleDNSMsg)
	if got == nil {
		t.Fatal("expected tables")
	}
	if len(got.Previous) != 4 || len(got.Current) != 4 {
		t.Fatalf("prev=%d curr=%d", len(got.Previous), len(got.Current))
	}
	if got.Previous[0] != (DNSChangeRow{Type: "A", Value: "147.79.69.222"}) {
		t.Fatalf("first prev=%+v", got.Previous[0])
	}
	if got.Current[1] != (DNSChangeRow{Type: "A", Value: "91.108.106.189"}) {
		t.Fatalf("second curr=%+v", got.Current[1])
	}
	if got.Summary() != "A and AAAA records changed" {
		t.Fatalf("summary=%q", got.Summary())
	}
}

func TestParseDNSChangeMessageRejectsPlainError(t *testing.T) {
	if ParseDNSChangeMessage("connection refused") != nil {
		t.Fatal("plain error should not parse")
	}
	if ParseDNSChangeMessage("") != nil {
		t.Fatal("empty should not parse")
	}
}
