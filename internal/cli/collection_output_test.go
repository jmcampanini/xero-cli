package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/config"
	"github.com/jmcampanini/xero-cli/internal/xero"
)

func TestCollectionCommandsExportJSONL(t *testing.T) {
	path := commandConfig(t)
	record := xero.Record{"Name": json.RawMessage(`"record"`), "Unknown": json.RawMessage(`9007199254740993`)}
	fake := &fakeClient{
		accounts:   []xero.Account{{AccountID: documentTestID, Code: "090", Name: "bank", Type: "BANK"}},
		categories: []xero.TrackingCategory{{Name: "category", Status: "ACTIVE"}},
		recordsFn: func(string, xero.ListQuery) (xero.RecordList, error) {
			return xero.RecordList{Complete: true, Items: []xero.Record{record}}, nil
		},
		attachmentsFn: func(string, string) ([]xero.Record, error) {
			return []xero.Record{record}, nil
		},
	}
	for _, tc := range []struct {
		args []string
		name string
	}{
		{[]string{"accounts", "list"}, "bank"},
		{[]string{"tracking-categories", "list"}, "category"},
		{[]string{"contacts", "list"}, "record"},
		{[]string{"bank-transactions", "list", "--account", "090", "--month", "2025-01"}, "record"},
		{[]string{"bank-transfers", "list", "--month", "2025-01"}, "record"},
		{[]string{"manual-journals", "list", "--month", "2025-01"}, "record"},
		{[]string{"bank-transaction", "attachments", documentTestID}, "record"},
		{[]string{"manual-journal", "attachments", documentTestID}, "record"},
	} {
		t.Run(strings.Join(tc.args[:2], " "), func(t *testing.T) {
			args := append([]string{"--config", path, "--color", "always"}, tc.args...)
			args = append(args, "--jsonl")
			code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
			var got map[string]json.RawMessage
			if code != 0 || stderr != "" || json.Unmarshal([]byte(out), &got) != nil || strings.Count(out, "\n") != 1 || strings.Contains(out, "\x1b") {
				t.Fatalf("JSONL = %d %q %q", code, out, stderr)
			}
			if string(got["Name"]) != `"`+tc.name+`"` {
				t.Errorf("JSONL Name = %s, want %q", got["Name"], tc.name)
			}

			args = append(args, "--csv")
			code, out, stderr = invoke(args, "", func(string, config.OrgConfig) client {
				t.Fatal("conflicting formats created a client")
				return nil
			})
			if code != 2 || out != "" || !strings.Contains(stderr, "cannot be combined") {
				t.Errorf("format conflict = %d %q %q", code, out, stderr)
			}
		})
	}
}

func TestCollectionOptionValidationBeforeConfiguration(t *testing.T) {
	for _, tc := range []struct {
		flags []string
		want  string
	}{
		{[]string{"--json", "--jsonl"}, "cannot be combined"},
		{[]string{"--no-header"}, "--no-header requires --csv"},
		{[]string{"--output-file", "records.json"}, "requires --json, --jsonl or --csv"},
		{[]string{"--jsonl", "--output-file", ""}, "--output-file must not be empty"},
		{[]string{"--overwrite"}, "--overwrite requires --output-file"},
	} {
		t.Run(strings.Join(tc.flags, " "), func(t *testing.T) {
			args := append([]string{"--config", filepath.Join(t.TempDir(), "missing"), "contacts", "list"}, tc.flags...)
			code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client {
				t.Fatal("invalid output options created a client")
				return nil
			})
			if code != 2 || out != "" || !strings.Contains(stderr, tc.want) {
				t.Errorf("invalid options = %d %q %q, want %q", code, out, stderr, tc.want)
			}
		})
	}
}

func TestCollectionOutputFileAndOverwrite(t *testing.T) {
	path := commandConfig(t)
	fake := &fakeClient{recordsFn: func(string, xero.ListQuery) (xero.RecordList, error) {
		return xero.RecordList{Complete: true, Items: []xero.Record{{"Name": json.RawMessage(`"record"`)}}}, nil
	}}
	factory := func(string, config.OrgConfig) client { return fake }
	for _, format := range []string{"--json", "--jsonl", "--csv"} {
		t.Run(format, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "records")
			args := []string{"--config", path, "contacts", "list", format, "--output-file", target}
			code, out, stderr := invoke(args, "", factory)
			body, err := os.ReadFile(target)
			if err != nil || code != 0 || out != "" || stderr != "" {
				t.Fatalf("file output = %d %q %q, read error %v", code, out, stderr, err)
			}
			if format == "--csv" {
				if string(body) != "Name\nrecord\n" {
					t.Errorf("CSV file = %q", body)
				}
			} else if !json.Valid(body) || !strings.Contains(string(body), `"Name":"record"`) || (format == "--json") != strings.Contains(string(body), `"complete":true`) {
				t.Errorf("JSON file = %s", body)
			}

			if err := os.WriteFile(target, []byte("keep original"), 0o600); err != nil {
				t.Fatal(err)
			}
			code, out, _ = invoke(args, "", factory)
			preserved, err := os.ReadFile(target)
			if code != 1 || out != "" || err != nil || string(preserved) != "keep original" {
				t.Fatalf("no overwrite = %d %q %q %v", code, out, preserved, err)
			}
			code, out, stderr = invoke(append(args, "--overwrite"), "", factory)
			replaced, err := os.ReadFile(target)
			if code != 0 || out != "" || stderr != "" || err != nil || string(replaced) != string(body) {
				t.Errorf("overwrite = %d %q %q %q %v", code, out, stderr, replaced, err)
			}
		})
	}
}

func TestCollectionFailuresPreserveOutputFile(t *testing.T) {
	path := commandConfig(t)
	for _, failure := range []string{"read", "encode"} {
		t.Run(failure, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "records")
			if err := os.WriteFile(target, []byte("keep original"), 0o600); err != nil {
				t.Fatal(err)
			}
			fake := &fakeClient{recordsFn: func(string, xero.ListQuery) (xero.RecordList, error) {
				if failure == "read" {
					return xero.RecordList{}, apperr.New("api", "page 2 failed")
				}
				return xero.RecordList{Items: []xero.Record{{"Name": json.RawMessage(`"valid"`)}, {"Name": json.RawMessage(`invalid`)}}}, nil
			}}
			code, out, _ := invoke([]string{"--config", path, "contacts", "list", "--jsonl", "--output-file", target, "--overwrite"}, "", func(string, config.OrgConfig) client { return fake })
			body, err := os.ReadFile(target)
			if code != 1 || out != "" || err != nil || string(body) != "keep original" {
				t.Errorf("failure = %d %q, destination %q, %v", code, out, body, err)
			}
		})
	}
}

func TestCollectionPageDiagnosticsAndHeaderlessCSV(t *testing.T) {
	path := commandConfig(t)
	fake := &fakeClient{recordsFn: func(string, xero.ListQuery) (xero.RecordList, error) {
		return xero.RecordList{Items: []xero.Record{{"Name": json.RawMessage(`"record"`)}}, Page: 2, PageCount: 3, ItemCount: 3}, nil
	}}
	for _, flags := range [][]string{{"--jsonl"}, {"--csv", "--no-header"}} {
		args := append([]string{"--config", path, "contacts", "list", "--page", "2"}, flags...)
		code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
		if code != 0 || !strings.Contains(stderr, "page 2 of 3 (3 items)") {
			t.Errorf("page export = %d %q %q", code, out, stderr)
		}
		if flags[0] == "--csv" && out != "record\n" || flags[0] == "--jsonl" && out != "{\"Name\":\"record\"}\n" {
			t.Errorf("page records = %q", out)
		}
	}
}
