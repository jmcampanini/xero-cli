package xero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

var xeroDate = regexp.MustCompile(`^/Date\((-?\d+)(?:[+-]\d{4})?\)/$`)

// ParseDate decodes a Xero /Date(milliseconds+offset)/ value in UTC.
// The optional offset is metadata; milliseconds already identify a UTC instant.
func ParseDate(value string) (time.Time, error) {
	match := xeroDate.FindStringSubmatch(value)
	if match == nil {
		return time.Time{}, fmt.Errorf("invalid Xero date %q", value)
	}
	ms, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse Xero date: %w", err)
	}
	return time.UnixMilli(ms).UTC(), nil
}

// NormalizeDates preserves JSON fields and numeric precision while converting Xero dates.
func NormalizeDates(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	normalizeValue(value)
	return json.Marshal(value)
}

func normalizeValue(value any) {
	switch object := value.(type) {
	case map[string]any:
		for key, value := range object {
			if text, ok := value.(string); ok {
				if normalized, ok := normalizeDate(key, text); ok {
					object[key] = normalized
				}
				continue
			}
			normalizeValue(value)
		}
	case []any:
		for _, item := range object {
			normalizeValue(item)
		}
	}
}

// normalizeDate converts a date-like field to a UTC calendar day, or to an RFC 3339
// UTC timestamp for UpdatedDateUTC and CreatedDateUTC. Unrecognized text is left as is.
func normalizeDate(key, text string) (string, bool) {
	timestamp := key == "UpdatedDateUTC" || key == "CreatedDateUTC"
	if !timestamp && !strings.HasPrefix(text, "/Date(") && !strings.HasPrefix(key, "Date") {
		return "", false
	}
	date, err := ParseDate(text)
	if err != nil {
		date, err = time.Parse(time.RFC3339Nano, text)
	}
	if err != nil {
		date, err = time.Parse("2006-01-02T15:04:05", text)
	}
	if err != nil {
		return "", false
	}
	if timestamp {
		return date.UTC().Format(time.RFC3339Nano), true
	}
	return date.UTC().Format("2006-01-02"), true
}

// ParseCalendarDate validates an explicit YYYY-MM-DD date without timezone shifts.
func ParseCalendarDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value || date.Year() < 1 {
		return time.Time{}, apperr.New("invalid_argument", "invalid date %q; expected YYYY-MM-DD", value)
	}
	return date, nil
}
