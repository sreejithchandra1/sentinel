package api

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var statsPeriodRe = regexp.MustCompile(`^([1-9][0-9]*)([mhdwM])$`)

const (
	defaultStatsPeriod = 24 * time.Hour
	minStatsPeriod     = time.Minute
	maxStatsPeriod     = 366 * 24 * time.Hour
)

type statsRange struct {
	From   time.Time
	To     time.Time
	Period string
}

func parseStatsRange(period, fromStr, toStr string) statsRange {
	now := time.Now().UTC()
	fromT, fromOK := parseStatsTime(fromStr)
	toT, toOK := parseStatsTime(toStr)
	if fromOK && toOK && toT.After(fromT) {
		if toT.Sub(fromT) > maxStatsPeriod {
			fromT = toT.Add(-maxStatsPeriod)
		}
		if toT.Sub(fromT) < minStatsPeriod {
			fromT = toT.Add(-minStatsPeriod)
		}
		return statsRange{From: fromT.UTC(), To: toT.UTC(), Period: "custom"}
	}
	period = normalizeStatsPeriod(period)
	return statsRange{
		From:   relativeSince(period, now),
		To:     now,
		Period: period,
	}
}

func parseStatsTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02T15:04:05", raw); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02 15:04", raw); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

func normalizeStatsPeriod(period string) string {
	period = strings.TrimSpace(period)
	if statsPeriodRe.MatchString(period) {
		return period
	}
	return "24h"
}

func relativeSince(period string, now time.Time) time.Time {
	m := statsPeriodRe.FindStringSubmatch(strings.TrimSpace(period))
	if m == nil {
		return now.Add(-defaultStatsPeriod)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return now.Add(-defaultStatsPeriod)
	}
	var from time.Time
	switch m[2] {
	case "m":
		from = now.Add(-time.Duration(n) * time.Minute)
	case "h":
		from = now.Add(-time.Duration(n) * time.Hour)
	case "d":
		from = now.AddDate(0, 0, -n)
	case "w":
		from = now.AddDate(0, 0, -7*n)
	case "M":
		from = now.AddDate(0, -n, 0)
	default:
		from = now.Add(-defaultStatsPeriod)
	}
	if now.Sub(from) > maxStatsPeriod {
		return now.Add(-maxStatsPeriod)
	}
	if now.Sub(from) < minStatsPeriod {
		return now.Add(-minStatsPeriod)
	}
	return from
}

func parseStatsPeriod(period string) time.Duration {
	now := time.Now().UTC()
	return now.Sub(relativeSince(normalizeStatsPeriod(period), now))
}

func statsSince(period string) time.Time {
	return relativeSince(normalizeStatsPeriod(period), time.Now().UTC())
}
