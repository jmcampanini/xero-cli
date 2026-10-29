package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestCollectionFormatsPreserveNativeFields(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"Name":"quoted, \"name\"\nnext","Amount":"1234.5000","Unknown":9007199254740993,"Tracking":[{"Name":"Region","Option":"North"}],"Null":null}`),
		json.RawMessage(`{"Name":"second","Later":true}`),
	}
	var jsonl bytes.Buffer
	if err := JSONL(&jsonl, items); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(jsonl.String(), "\n"), "\n")
	if len(lines) != len(items) || !strings.HasSuffix(jsonl.String(), "\n") {
		t.Fatalf("JSONL = %q, want two newline-terminated records", jsonl.String())
	}
	for i, line := range lines {
		var got, want map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(items[i], &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("JSONL record %d = %s, want %s", i, line, items[i])
		}
	}

	wantRows := [][]string{
		{"Amount", "Later", "Name", "Null", "Tracking", "Unknown"},
		{"1234.5000", "", "quoted, \"name\"\nnext", "", `[{"Name":"Region","Option":"North"}]`, "9007199254740993"},
		{"", "true", "second", "", "", ""},
	}
	for _, noHeader := range []bool{false, true} {
		var output bytes.Buffer
		if err := CSV(&output, items, noHeader); err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(&output).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		want := wantRows
		if noHeader {
			want = want[1:]
		}
		if !reflect.DeepEqual(rows, want) {
			t.Errorf("CSV(noHeader=%v) = %#v, want %#v", noHeader, rows, want)
		}
	}
}

func TestCollectionFormatsEmptyAndInvalidRecords(t *testing.T) {
	for name, render := range map[string]func(io.Writer, any) error{
		"jsonl": JSONL,
		"csv": func(w io.Writer, items any) error {
			return CSV(w, items, false)
		},
	} {
		t.Run(name, func(t *testing.T) {
			for _, items := range []any{nil, []json.RawMessage(nil), []json.RawMessage{}} {
				var output bytes.Buffer
				if err := render(&output, items); err != nil || output.Len() != 0 {
					t.Errorf("empty render = %q, %v", output.String(), err)
				}
			}
			for _, items := range []any{
				[]json.RawMessage{json.RawMessage(`{"Name":"valid"}`), json.RawMessage(`null`)},
				[]json.RawMessage{json.RawMessage(`{"Name":"valid"}`), json.RawMessage(`broken`)},
				[]string{"not an object"},
			} {
				var output bytes.Buffer
				if err := render(&output, items); err == nil || output.Len() != 0 {
					t.Errorf("invalid render = %q, %v, want error and no output", output.String(), err)
				}
			}
			failure := errors.New("destination failed")
			if err := render(collectionFailingWriter{failure}, []map[string]string{{"Name": "record"}}); !errors.Is(err, failure) {
				t.Errorf("write failure = %v, want %v", err, failure)
			}
		})
	}
}

type collectionFailingWriter struct{ err error }

func (w collectionFailingWriter) Write([]byte) (int, error) { return 0, w.err }
