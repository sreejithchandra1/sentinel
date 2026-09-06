package alerter

import (
	"fmt"
	"regexp"
	"strings"
)

type DNSChangeRow struct {
	Type  string
	Value string
}

type DNSChangeTables struct {
	Previous []DNSChangeRow
	Current  []DNSChangeRow
}

var dnsChangeSegment = regexp.MustCompile(`(?i)([A-Za-z0-9]+)\s+record changed from\s+(.+?)\s+to\s+(.+?)(?:;|$)`)

func ParseDNSChangeMessage(msg string) *DNSChangeTables {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil
	}
	matches := dnsChangeSegment.FindAllStringSubmatch(msg, -1)
	if len(matches) == 0 {
		return nil
	}
	out := &DNSChangeTables{}
	for _, m := range matches {
		recType := strings.ToUpper(strings.TrimSpace(m[1]))
		for _, v := range splitDNSValues(m[2]) {
			out.Previous = append(out.Previous, DNSChangeRow{Type: recType, Value: v})
		}
		for _, v := range splitDNSValues(m[3]) {
			out.Current = append(out.Current, DNSChangeRow{Type: recType, Value: v})
		}
	}
	if len(out.Previous) == 0 && len(out.Current) == 0 {
		return nil
	}
	return out
}

func (t *DNSChangeTables) Summary() string {
	if t == nil {
		return ""
	}
	seen := map[string]bool{}
	var types []string
	for _, row := range append(append([]DNSChangeRow{}, t.Previous...), t.Current...) {
		if row.Type == "" || seen[row.Type] {
			continue
		}
		seen[row.Type] = true
		types = append(types, row.Type)
	}
	if len(types) == 0 {
		return "DNS records changed"
	}
	if len(types) == 1 {
		return fmt.Sprintf("%s record changed", types[0])
	}
	if len(types) == 2 {
		return fmt.Sprintf("%s and %s records changed", types[0], types[1])
	}
	return fmt.Sprintf("%s, and %s records changed", strings.Join(types[:len(types)-1], ", "), types[len(types)-1])
}

func splitDNSValues(raw string) []string {
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
