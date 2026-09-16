package xero

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

func TestRecordedDocumentsPreserveFields(t *testing.T) {
	// Recorded September 16, 2026; all identities, text, dates and nonzero amounts
	// are anonymized. Retaining optional fields catches source-shape assumptions.
	for _, test := range []struct{ file, resource string }{
		{"bank-transactions", "BankTransactions"}, {"bank-transfers", "BankTransfers"},
		{"manual-journals", "ManualJournals"}, {"contacts", "Contacts"},
	} {
		t.Run(test.file, func(t *testing.T) {
			body, err := os.ReadFile("testdata/documents/" + test.file + ".json")
			if err != nil {
				t.Fatal(err)
			}
			original, err := decodeResource[Record](body, test.resource)
			if err != nil {
				t.Fatal(err)
			}
			items, err := normalizeRecords(body, test.resource)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != len(original) {
				t.Fatalf("record count = %d, want %d", len(items), len(original))
			}
			for i := range items {
				assertRecordFields(t, original[i], items[i])
			}
		})
	}
}

func assertRecordFields(t *testing.T, want, got Record) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("field count = %d, want %d", len(got), len(want))
	}
	for key, raw := range want {
		value, ok := got[key]
		if !ok {
			t.Errorf("dropped %s", key)
			continue
		}
		if len(raw) > 0 && raw[0] == '{' {
			assertRecordFields(t, want.Object(key), got.Object(key))
		}
		if len(raw) > 0 && raw[0] == '[' {
			var before, after []json.RawMessage
			if err := json.Unmarshal(raw, &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(value, &after); err != nil {
				t.Fatal(err)
			}
			if len(before) != len(after) {
				t.Errorf("%s array count changed", key)
				continue
			}
			for i, item := range before {
				if len(item) > 0 && item[0] == '{' {
					var a, b Record
					if err := json.Unmarshal(item, &a); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(after[i], &b); err != nil {
						t.Fatal(err)
					}
					assertRecordFields(t, a, b)
				}
			}
		}
	}
}

func TestRecordsPaginationAndCompleteness(t *testing.T) {
	for _, resource := range []string{"BankTransactions", "ManualJournals", "Contacts"} {
		for _, single := range []bool{false, true} {
			t.Run(resource+strconv.FormatBool(single), func(t *testing.T) {
				idKey, _ := recordID(resource)
				var pages []string
				c := testClient(t, map[string]http.HandlerFunc{"/api/" + resource: func(w http.ResponseWriter, r *http.Request) {
					pages = append(pages, r.URL.Query().Get("page"))
					page, _ := strconv.Atoi(r.URL.Query().Get("page"))
					order := "Date," + idKey
					if resource == "Contacts" {
						order = "Name," + idKey
					}
					if r.URL.Query().Get("order") != order || r.URL.Query().Get("pageSize") != "100" {
						t.Errorf("query = %v", r.URL.Query())
					}
					if resource == "BankTransactions" && r.URL.Query().Get("unitdp") != "4" {
						t.Error("missing unitdp")
					}
					if r.Header.Get("If-Modified-Since") != "2025-01-01T00:00:00" {
						t.Error("missing modified-since")
					}
					var batch []json.RawMessage
					for index := (page - 1) * 100; index < min(page*100, 201); index++ {
						batch = append(batch, json.RawMessage(fmt.Sprintf(`{%q:%q,"Date":"/Date(1735689600000+1300)/","Total":9007199254740993.1200}`, idKey, strconv.Itoa(index))))
					}
					encoded, _ := json.Marshal(batch)
					_, _ = fmt.Fprintf(w, `{"pagination":{"page":%d,"pageSize":100,"pageCount":3,"itemCount":201},%q:%s}`, page, resource, encoded)
				}})
				query := ListQuery{ModifiedSince: "2025-01-01T00:00:00"}
				if single {
					query.Page = 2
				}
				got, err := c.Records(context.Background(), resource, query)
				if err != nil {
					t.Fatal(err)
				}
				wantPages, wantCount := []string{"1", "2", "3"}, 201
				if single {
					wantPages, wantCount = []string{"2"}, 100
				}
				if !reflect.DeepEqual(pages, wantPages) || len(got.Items) != wantCount || got.Complete == single {
					t.Errorf("pages %v, list %+v", pages, got)
				}
				for index, item := range got.Items {
					wantID := index
					if single {
						wantID += 100
					}
					if item.Text(idKey) != strconv.Itoa(wantID) {
						t.Errorf("record %d ID = %q, want server order ID %d", index, item.Text(idKey), wantID)
					}
				}
				if got.Items[0].Text("Date") != "2025-01-01" || string(got.Items[0]["Total"]) != `"9007199254740993.1200"` {
					t.Errorf("normalized item = %s", got.Items[0])
				}
			})
		}
	}
}

func TestRecordsRejectUnverifiableCollection(t *testing.T) {
	for _, fault := range []string{"missing pagination", "changed count", "changed pages", "duplicate ID", "missing ID", "short collection", "HTTP failure", "invalid money"} {
		t.Run(fault, func(t *testing.T) {
			c := testClient(t, map[string]http.HandlerFunc{"/api/Contacts": func(w http.ResponseWriter, r *http.Request) {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				pages, count, id := 2, 2, strconv.Itoa(page)
				if fault == "missing pagination" {
					_, _ = fmt.Fprint(w, `{"Contacts":[]}`)
					return
				}
				if fault == "missing ID" {
					id = ""
				}
				if fault == "duplicate ID" {
					id = "same"
				}
				if fault == "short collection" {
					pages = 1
				}
				if fault == "invalid money" {
					_, _ = fmt.Fprint(w, `{"Contacts":[{"Total":"NaN"}]}`)
					return
				}
				if page == 2 {
					switch fault {
					case "changed count":
						count = 3
					case "changed pages":
						pages = 3
					case "HTTP failure":
						w.WriteHeader(500)
						return
					}
				}
				_, _ = fmt.Fprintf(w, `{"pagination":{"page":%d,"pageSize":100,"pageCount":%d,"itemCount":%d},"Contacts":[{"ContactID":%q}]}`, page, pages, count, id)
			}})
			got, err := c.Records(context.Background(), "Contacts", ListQuery{})
			if err == nil || len(got.Items) != 0 || got.Complete {
				t.Errorf("list %+v, error %v", got, err)
			}
		})
	}
}

func TestRecordsEmptyAndUnpaged(t *testing.T) {
	for _, resource := range []string{"Contacts", "BankTransfers"} {
		t.Run(resource, func(t *testing.T) {
			c := testClient(t, map[string]http.HandlerFunc{"/api/" + resource: func(w http.ResponseWriter, r *http.Request) {
				if resource == "Contacts" {
					_, _ = fmt.Fprint(w, `{"pagination":{"page":1,"pageSize":100,"pageCount":0,"itemCount":0},"Contacts":[]}`)
				} else {
					if r.URL.Query().Has("page") || r.URL.Query().Has("pageSize") {
						t.Error("bank transfer pagination sent")
					}
					_, _ = fmt.Fprint(w, `{"BankTransfers":[]}`)
				}
			}})
			got, err := c.Records(context.Background(), resource, ListQuery{})
			if err != nil || !got.Complete || got.Items == nil || len(got.Items) != 0 {
				t.Errorf("list %+v, error %v", got, err)
			}
		})
	}
}

func TestRecordPreservesTrackingAndUnknownFields(t *testing.T) {
	const raw = `{"BankTransactions":[{"BankTransactionID":"id","Date":"/Date(1735689600000-0800)/","UpdatedDateUTC":"/Date(1735734896000+1300)/","Total":100.1200,"UnknownNumber":9007199254740993,"LineItems":[{"Quantity":1.5,"UnitAmount":66.7467,"TaxAmount":0.00,"LineAmount":100.1200,"Tracking":[{"Name":"Region","Option":"North","TrackingCategoryID":"category","TrackingOptionID":"option","UnknownFlag":true}]}],"UnknownArray":[1,"two",null]}]}`
	c := testClient(t, map[string]http.HandlerFunc{"/api/BankTransactions/id": func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("unitdp") != "4" {
			t.Error("show lost unit amount precision")
		}
		_, _ = fmt.Fprint(w, raw)
	}})
	got, err := c.Record(context.Background(), "BankTransactions", "id")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"Date":"2025-01-01"`, `"UpdatedDateUTC":"2025-01-01T12:34:56Z"`, `"Total":"100.1200"`, `"UnitAmount":"66.7467"`, `"Quantity":1.5`, `"UnknownNumber":9007199254740993`, `"UnknownFlag":true`, `"Name":"Region"`, `"Option":"North"`, `"TrackingCategoryID":"category"`, `"TrackingOptionID":"option"`, `"UnknownArray":[1,"two",null]`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("missing %s in %s", want, encoded)
		}
	}
	for _, alias := range []string{`"category":`, `"option":`, `"category_id":`, `"option_id":`} {
		if strings.Contains(string(encoded), alias) {
			t.Errorf("unexpected alias %s", alias)
		}
	}
}

func TestRecordNotFoundNamesID(t *testing.T) {
	c := testClient(t, map[string]http.HandlerFunc{"/api/Contacts/missing": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) }})
	_, err := c.Record(context.Background(), "Contacts", "missing")
	if err == nil || apperr.From(err).Code != "not_found" || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestAttachmentMissingScopeIsNamed(t *testing.T) {
	c := testClient(t, map[string]http.HandlerFunc{"/api/ManualJournals/id/Attachments": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }})
	_, err := c.Attachments(context.Background(), "ManualJournals", "id")
	if err == nil || apperr.From(err).Code != "forbidden" || !strings.Contains(err.Error(), "accounting.attachments.read") {
		t.Errorf("attachment scope error = %v", err)
	}
}

func TestDocumentReadScopesAreIndependent(t *testing.T) {
	for _, test := range []struct{ scope, group string }{
		{"accounting.banktransactions.read", "bank-transactions"},
		{"accounting.banktransactions", "bank-transfers"},
		{"accounting.manualjournals.read", "manual-journals"},
		{"accounting.transactions.read", "manual-journals"},
		{"accounting.contacts.read", "contacts"},
		{"accounting.attachments.read", "attachments"},
		{"accounting.attachments", "attachments"},
	} {
		if !Capabilities([]string{test.scope})[test.group] {
			t.Errorf("%s did not enable %s", test.scope, test.group)
		}
	}
	if Capabilities([]string{"accounting.banktransactions.read"})["manual-journals"] || Capabilities([]string{"accounting.transactions.read"})["attachments"] {
		t.Error("unrelated scopes enabled reads")
	}
}

func TestAttachmentUsesListedMIMEAndEscapedName(t *testing.T) {
	name := "bill & tax #1.pdf"
	want := []byte{0, 1, 2, 255, '\n', 3}
	c := testClient(t, map[string]http.HandlerFunc{
		"/api/BankTransactions/id/Attachments": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"Attachments":[{"FileName":%q,"MimeType":"application/pdf","ContentLength":6,"Unknown":true}]}`, name)
		},
		"/api/BankTransactions/id/Attachments/" + url.PathEscape(name): func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "application/pdf" || r.URL.RawQuery != "" {
				t.Errorf("download request = %v, %v", r.URL, r.Header.Get("Accept"))
			}
			_, _ = w.Write(want)
		},
	})
	items, err := c.Attachments(context.Background(), "BankTransactions", "id")
	if err != nil || len(items) != 1 || items[0].Text("Unknown") != "true" {
		t.Fatalf("attachments %v, %v", items, err)
	}
	got, err := c.Attachment(context.Background(), "BankTransactions", "id", name)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("download = %v, %v", got, err)
	}
	_, err = c.Attachment(context.Background(), "BankTransactions", "id", "absent")
	if err == nil || apperr.From(err).Code != "not_found" {
		t.Errorf("missing error %v", err)
	}
}

func TestJournalDebitsExact(t *testing.T) {
	lines := []Record{{"LineAmount": json.RawMessage(`"9007199254740993.1234"`)}, {"LineAmount": json.RawMessage(`"0.0066"`)}, {"LineAmount": json.RawMessage(`"-1.00"`)}, {"IsBlank": json.RawMessage(`true`)}}
	got, err := JournalDebits(lines)
	if err != nil || got != "9007199254740993.1300" {
		t.Errorf("debits %q, %v", got, err)
	}
	for _, value := range []string{"", "1e3", "NaN", "1,000", ".5", "1.", "1/2", "+1"} {
		if ValidDecimal(value) {
			t.Errorf("accepted %q", value)
		}
	}
	for _, value := range []string{"0", "-0.00", "100.1234", "-12.5"} {
		if !ValidDecimal(value) {
			t.Errorf("rejected %q", value)
		}
	}
}

func TestRecordsDoesNotMutateQuery(t *testing.T) {
	query := url.Values{"where": {"Name==\"Example\""}}
	c := testClient(t, map[string]http.HandlerFunc{"/api/Contacts": func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"pagination":{"page":1,"pageSize":100,"pageCount":0,"itemCount":0},"Contacts":[]}`)
	}})
	_, err := c.Records(context.Background(), "Contacts", ListQuery{Values: query})
	if err != nil || len(query) != 1 {
		t.Errorf("query %v, error %v", query, err)
	}
}
