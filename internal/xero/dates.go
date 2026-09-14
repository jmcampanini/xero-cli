package xero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseDate decodes a Xero /Date(milliseconds+offset)/ value in UTC.
// The optional offset is metadata; milliseconds already identify a UTC instant.
func ParseDate(value string) (time.Time, error) {
	match := regexp.MustCompile(`^/Date\((-?\d+)(?:[+-]\d{4})?\)/$`).FindStringSubmatch(value)
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
			text, ok := value.(string)
			if ok && (strings.HasPrefix(text, "/Date(") || strings.HasPrefix(key, "Date") || key == "UpdatedDateUTC" || key == "CreatedDateUTC") {
				date, err := ParseDate(text)
				if err != nil {
					date, err = time.Parse(time.RFC3339Nano, text)
				}
				if err != nil {
					date, err = time.Parse("2006-01-02T15:04:05", text)
				}
				if err == nil {
					layout := "2006-01-02"
					if key == "UpdatedDateUTC" || key == "CreatedDateUTC" {
						layout = time.RFC3339Nano
					}
					object[key] = date.UTC().Format(layout)
				}
			} else {
				normalizeValue(value)
			}
		}
	case []any:
		for _, item := range object {
			normalizeValue(item)
		}
	}
}
