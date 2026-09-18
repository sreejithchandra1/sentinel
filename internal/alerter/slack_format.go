package alerter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildSlackPayload creates a Block Kit attachment matching the Sentinel alert design.
func buildSlackPayload(meta AlertMeta) ([]byte, error) {
	statusEmoji := ":red_circle:"
	switch meta.StatusLabel() {
	case "UP", "OK":
		statusEmoji = ":large_green_circle:"
	case "SLOW", "WARNING":
		statusEmoji = ":large_yellow_circle:"
	}

	var fields []map[string]any
	if downtime := meta.DowntimeLabel(); downtime != "" {
		fields = []map[string]any{
			mrkdwnField("*Downtime*\n" + downtime),
			mrkdwnField("*Response*\n" + meta.ResponseLabel()),
			mrkdwnField("*" + meta.TimeFieldLabel() + "*\n" + meta.EventTimeLabel()),
			mrkdwnField("*Incident*\n`" + meta.IncidentLabel() + "`"),
		}
	} else {
		fields = []map[string]any{
			mrkdwnField("*Status*\n" + statusEmoji + " `" + meta.StatusLabel() + "`"),
			mrkdwnField("*Response*\n" + meta.ResponseLabel()),
			mrkdwnField("*" + meta.TimeFieldLabel() + "*\n" + meta.EventTimeLabel()),
			mrkdwnField("*Incident*\n`" + meta.IncidentLabel() + "`"),
		}
	}

	bodyText := fmt.Sprintf("*%s*", meta.Name)
	if meta.URL != "" {
		bodyText += fmt.Sprintf("\n<%s|%s>", meta.URL, meta.URL)
	}
	ev := strings.ToUpper(meta.Event)
	if meta.Message != "" && ev != "NORMAL" {
		if meta.ResponseLabel() != "Timeout" {
			if dns := ParseDNSChangeMessage(meta.Message); dns != nil {
				bodyText += "\n" + formatSlackDNSChanges(dns)
			} else if rows := ParseHostServiceMessage(meta.Message); len(rows) > 0 {
				bodyText += "\n" + formatSlackHostServices(rows)
			} else {
				bodyText += fmt.Sprintf("\n_%s_", meta.Message)
			}
		}
	}

	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": meta.Title(), "emoji": true},
		},
		{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": bodyText},
		},
		{
			"type":   "section",
			"fields": fields,
		},
	}
	if meta.DashboardURL != "" {
		elements := []map[string]any{
			{
				"type": "button",
				"text": map[string]any{
					"type":  "plain_text",
					"text":  "Open in Sentinel →",
					"emoji": true,
				},
				"url":   meta.DashboardURL,
				"style": "primary",
			},
		}
		if meta.ErrorPageURL != "" {
			elements = append(elements, map[string]any{
				"type": "button",
				"text": map[string]any{
					"type":  "plain_text",
					"text":  "View captured page",
					"emoji": true,
				},
				"url": meta.ErrorPageURL,
			})
		}
		blocks = append(blocks, map[string]any{
			"type":     "actions",
			"elements": elements,
		})
	}

	payload := map[string]any{
		"text": meta.FallbackText(),
		"attachments": []map[string]any{
			{
				"color":  meta.Color(),
				"blocks": blocks,
			},
		},
	}
	return json.Marshal(payload)
}

func formatSlackDNSChanges(t *DNSChangeTables) string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("*Previous*\n")
	if len(t.Previous) == 0 {
		b.WriteString("—\n")
	} else {
		for _, row := range t.Previous {
			b.WriteString(fmt.Sprintf("`%s` %s\n", row.Type, row.Value))
		}
	}
	b.WriteString("*Current*\n")
	if len(t.Current) == 0 {
		b.WriteString("—")
	} else {
		for i, row := range t.Current {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(fmt.Sprintf("`%s` %s", row.Type, row.Value))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatSlackHostServices(rows []HostServiceRow) string {
	var b strings.Builder
	b.WriteString("*Services*\n")
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(fmt.Sprintf("`%s` %s", row.Name, row.Status))
	}
	return b.String()
}

func mrkdwnField(text string) map[string]any {
	return map[string]any{
		"type": "mrkdwn",
		"text": text,
	}
}
