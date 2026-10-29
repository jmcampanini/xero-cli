package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
)

// JSONL writes one native JSON object per line; an empty collection writes nothing.
func JSONL(w io.Writer, items any) error {
	records, err := collectionRecords(items)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := JSON(w, record); err != nil {
			return err
		}
	}
	return nil
}

// CSV writes the sorted union of native fields, with nested values as compact JSON.
// Missing and null fields are empty cells; an empty collection writes nothing.
func CSV(w io.Writer, items any, noHeader bool) error {
	records, err := collectionRecords(items)
	if err != nil {
		return err
	}
	fields := make(map[string]bool)
	for _, record := range records {
		for field := range record {
			fields[field] = true
		}
	}
	columns := slices.Sorted(maps.Keys(fields))
	if len(columns) == 0 {
		return nil
	}

	writer := csv.NewWriter(w)
	if !noHeader {
		if err := writer.Write(columns); err != nil {
			return err
		}
	}
	for _, record := range records {
		row := make([]string, len(columns))
		for i, field := range columns {
			raw := record[field]
			if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
				continue
			}
			if raw[0] == '"' {
				if err := json.Unmarshal(raw, &row[i]); err != nil {
					return fmt.Errorf("decode CSV field %q: %w", field, err)
				}
			} else {
				row[i] = string(raw)
			}
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func collectionRecords(items any) ([]map[string]json.RawMessage, error) {
	body, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("encode collection: %w", err)
	}
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("decode collection records: %w", err)
	}
	for _, record := range records {
		if record == nil {
			return nil, fmt.Errorf("collection record must be a JSON object")
		}
	}
	return records, nil
}
