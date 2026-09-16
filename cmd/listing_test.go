package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/config"
	"github.com/jmcampanini/xero-cli/internal/xero"
)

const documentTestID = "00000000-0000-0000-0000-000000000001"

func TestDocumentFilters(t *testing.T) {
	path := commandConfig(t)
	const period = "Date>=DateTime(2024,2,1)&&Date<=DateTime(2024,2,29)"
	const account = `BankAccount.AccountID==Guid("` + documentTestID + `")`
	for _, test := range []struct {
		name                      string
		args                      []string
		resource, where, modified string
		values                    map[string]string
	}{
		{"bank default", []string{"bank-transactions", "list", "--account", "090", "--month", "2024-02"}, "BankTransactions", period + "&&" + account + `&&Status=="AUTHORISED"`, "", nil},
		{"bank all flags", []string{"bank-transactions", "list", "--account", "090", "--month", "2024-02", "--type", "transfer", "--contact", documentTestID, "--reference", `a"b\c`, "--amount", "12.3400", "--unreconciled", "--include-deleted", "--where", `Total>1||Total<0`}, "BankTransactions", period + "&&" + account + `&&(Type=="SPEND-TRANSFER"||Type=="RECEIVE-TRANSFER")&&Contact.ContactID==Guid("` + documentTestID + `")&&Reference=="a\"b\\c"&&Total==12.3400&&IsReconciled==false&&(Total>1||Total<0)`, "", nil},
		{"bank spend", []string{"bank-transactions", "list", "--account", "090", "--month", "2024-02", "--type", "spend"}, "BankTransactions", period + "&&" + account + `&&Status=="AUTHORISED"&&Type=="SPEND"`, "", nil},
		{"bank receive", []string{"bank-transactions", "list", "--account", "090", "--month", "2024-02", "--type", "receive"}, "BankTransactions", period + "&&" + account + `&&Status=="AUTHORISED"&&Type=="RECEIVE"`, "", nil},
		{"bank empty reference", []string{"bank-transactions", "list", "--account", "090", "--month", "2024-02", "--reference", ""}, "BankTransactions", period + "&&" + account + `&&Status=="AUTHORISED"&&Reference==""`, "", nil},
		{"transfers", []string{"bank-transfers", "list", "--from", "2024-02-01", "--to", "2024-02-29"}, "BankTransfers", period, "", nil},
		{"journals default", []string{"manual-journals", "list", "--month", "2024-02"}, "ManualJournals", period + `&&Status=="POSTED"`, "", nil},
		{"journals voided", []string{"manual-journals", "list", "--month", "2024-02", "--status", "voided"}, "ManualJournals", period + `&&Status=="VOIDED"`, "", nil},
		{"journals draft", []string{"manual-journals", "list", "--month", "2024-02", "--status", "draft"}, "ManualJournals", period + `&&Status=="DRAFT"`, "", nil},
		{"journals year", []string{"manual-journals", "list", "--year", "2025", "--status", "all", "--modified-since", "2025-01-01T03:04:05+02:00", "--where", `Narration=="Test"`}, "ManualJournals", `Date>=DateTime(2025,1,1)&&Date<=DateTime(2025,12,31)&&(Narration=="Test")`, "2025-01-01T01:04:05", nil},
		{"contacts default", []string{"contacts", "list"}, "Contacts", `ContactStatus=="ACTIVE"`, "", map[string]string{"includeArchived": "false"}},
		{"contacts combined", []string{"contacts", "list", "--customers", "--suppliers", "--status", "archived", "--search", "a & b", "--where", `Name!="x"`}, "Contacts", `ContactStatus=="ARCHIVED"&&IsCustomer==true&&IsSupplier==true&&(Name!="x")`, "", map[string]string{"includeArchived": "true", "searchTerm": "a & b"}},
		{"contacts all", []string{"contacts", "list", "--status", "all"}, "Contacts", "", "", map[string]string{"includeArchived": "true"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			fake := &fakeClient{accounts: []xero.Account{{AccountID: documentTestID, Code: "090", Type: "BANK"}}}
			fake.recordsFn = func(resource string, query xero.ListQuery) (xero.RecordList, error) {
				called = true
				if resource != test.resource || query.Values.Get("where") != test.where || query.ModifiedSince != test.modified {
					t.Errorf("resource/query = %s %+v, want where %s", resource, query, test.where)
				}
				for key, want := range test.values {
					if query.Values.Get(key) != want {
						t.Errorf("%s = %s, want %s", key, query.Values.Get(key), want)
					}
				}
				if query.Values.Has("summaryOnly") {
					t.Error("contact summary loses roles")
				}
				return xero.RecordList{Complete: true}, nil
			}
			args := append([]string{"--config", path, "--json"}, test.args...)
			code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
			if code != 0 || stderr != "" || !called || !strings.Contains(out, `"items":[]`) {
				t.Errorf("result %d %q %q, called %v", code, out, stderr, called)
			}
		})
	}
}

func TestDocumentValidationBeforeConfig(t *testing.T) {
	for _, args := range [][]string{
		{"bank-transactions", "list", "--month", "2025-01"},
		{"bank-transactions", "list", "--account", "090", "--month", "2025-01", "--type", "invalid"},
		{"bank-transactions", "list", "--account", "090", "--month", "2025-01", "--contact", "name"},
		{"bank-transactions", "list", "--account", "090", "--month", "2025-01", "--amount", "1e3"},
		{"manual-journals", "list", "--month", "2025-01", "--status", "invalid"},
		{"manual-journals", "list", "--year", "2025", "--modified-since", "yesterday"},
		{"contacts", "list", "--status", "deleted"}, {"contacts", "list", "--where", " "},
		{"contacts", "list", "--page", "0"}, {"contacts", "list", "--page-size", "1"}, {"contacts", "list", "--page", "1", "--page-size", "1001"},
		{"bank-transfers", "list"}, {"bank-transfers", "list", "--from", "2025-01-01"},
		{"bank-transfers", "list", "--month", "2025-13"}, {"bank-transfers", "list", "--month", "2025-01", "--from", "2025-01-01", "--to", "2025-01-31"},
		{"bank-transfers", "list", "--from", "2025-02-01", "--to", "2025-01-01"},
		{"bank-transaction", "show", "invalid"}, {"bank-transfer", "show", "invalid"}, {"manual-journal", "show", "invalid"}, {"contact", "show", "invalid"},
		{"manual-journal", "attachments", "invalid"}, {"bank-transaction", "attachment", documentTestID, "../escape"},
		{"bank-transaction", "attachment", documentTestID, "file", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, out, stderr := invoke(append([]string{"--config", "/missing/document-config"}, args...), "", func(string, config.OrgConfig) client { t.Fatal("client created for invalid args"); return nil })
			if code == 0 || out != "" || strings.Contains(stderr, "document-config") {
				t.Errorf("result %d %q %q", code, out, stderr)
			}
		})
	}
}

func TestDocumentPagingOutputAndFailure(t *testing.T) {
	path := commandConfig(t)
	for _, test := range []struct {
		resource string
		args     []string
		complete bool
		note     string
	}{
		{"Contacts", []string{"contacts", "list", "--page", "2", "--page-size", "7"}, false, "page 2 of 3 (20 items)"},
		{"BankTransfers", []string{"bank-transfers", "list", "--month", "2025-01", "--page", "2", "--page-size", "7"}, true, "not paginated"},
	} {
		fake := &fakeClient{recordsFn: func(resource string, query xero.ListQuery) (xero.RecordList, error) {
			if resource != test.resource || query.Page != 2 || query.PageSize != 7 {
				t.Errorf("query %+v", query)
			}
			return xero.RecordList{Complete: test.complete, Page: 2, PageCount: 3, ItemCount: 20}, nil
		}}
		code, out, stderr := invoke(append([]string{"--config", path, "--json"}, test.args...), "", func(string, config.OrgConfig) client { return fake })
		var value struct{ Complete bool }
		if err := json.Unmarshal([]byte(out), &value); err != nil {
			t.Fatal(err)
		}
		if code != 0 || value.Complete != test.complete || !strings.Contains(stderr, test.note) {
			t.Errorf("result %d %q %q", code, out, stderr)
		}
	}
	fake := &fakeClient{recordsFn: func(string, xero.ListQuery) (xero.RecordList, error) {
		return xero.RecordList{}, apperr.New("api", "page 2 failed")
	}}
	code, out, stderr := invoke([]string{"--config", path, "contacts", "list"}, "", func(string, config.OrgConfig) client { return fake })
	if code != 1 || out != "" || !strings.Contains(stderr, "page 2 failed") {
		t.Errorf("result %d %q %q", code, out, stderr)
	}
}

func TestBankAccountResolutionWithoutCode(t *testing.T) {
	path := commandConfig(t)
	for _, kind := range []string{"BANK", "REVENUE"} {
		fake := &fakeClient{accounts: []xero.Account{{AccountID: documentTestID, Name: "Example Bank", Type: kind}}}
		code, out, stderr := invoke([]string{"--config", path, "bank-transactions", "list", "--account", "Example Bank", "--month", "2025-01"}, "", func(string, config.OrgConfig) client { return fake })
		if kind == "BANK" && code != 0 || kind != "BANK" && (code != 1 || out != "") {
			t.Errorf("%s result %d %q %q", kind, code, out, stderr)
		}
	}
}

func TestAttachmentFilesAndStdout(t *testing.T) {
	path := commandConfig(t)
	dir := t.TempDir()
	t.Chdir(dir)
	want := []byte{0, 1, 255, 3}
	for _, group := range []string{"bank-transaction", "manual-journal"} {
		t.Run(group, func(t *testing.T) {
			fake := &fakeClient{attachmentFn: func(_, id, name string) ([]byte, error) {
				if id != documentTestID || name != "bill.pdf" {
					t.Errorf("download %s %s", id, name)
				}
				return want, nil
			}}
			factory := func(string, config.OrgConfig) client { return fake }
			base := []string{"--config", path, group, "attachment", documentTestID, "bill.pdf"}
			code, out, stderr := invoke(append(append([]string{}, base...), "--output", "-"), "", factory)
			if code != 0 || !reflect.DeepEqual([]byte(out), want) || stderr != "" {
				t.Errorf("stdout %d %q %q", code, out, stderr)
			}
			target := filepath.Join(dir, group+".pdf")
			code, out, stderr = invoke(append(append([]string{}, base...), "--output", target), "", factory)
			body, err := os.ReadFile(target)
			if err != nil || code != 0 || out != "" || stderr != "" || !reflect.DeepEqual(body, want) {
				t.Fatalf("file %d %q %q %v", code, out, stderr, err)
			}
			code, out, stderr = invoke(append(append([]string{}, base...), "--output", target), "", func(string, config.OrgConfig) client { t.Fatal("existing file triggered client creation"); return nil })
			if code != 1 || out != "" || !strings.Contains(stderr, "overwrite") {
				t.Errorf("overwrite %d %q %q", code, out, stderr)
			}
			body, _ = os.ReadFile(target)
			if !reflect.DeepEqual(body, want) {
				t.Error("existing file changed")
			}
		})
	}
	fake := &fakeClient{attachmentFn: func(_, _, _ string) ([]byte, error) { return want, nil }}
	code, out, stderr := invoke([]string{"--config", path, "bank-transaction", "attachment", documentTestID, "-"}, "", func(string, config.OrgConfig) client { return fake })
	body, err := os.ReadFile(filepath.Join(dir, "-"))
	if code != 0 || out != "" || stderr != "" || err != nil || !reflect.DeepEqual(body, want) {
		t.Errorf("default filename %d %q %q %v", code, out, stderr, err)
	}
}

func TestShowAttachmentCountBestEffort(t *testing.T) {
	path := commandConfig(t)
	for _, group := range []string{"bank-transaction", "manual-journal"} {
		for _, failure := range []bool{false, true} {
			fake := &fakeClient{recordFn: func(string, string) (xero.Record, error) {
				return xero.Record{"HasAttachments": json.RawMessage(`true`)}, nil
			}, attachmentsFn: func(string, string) ([]xero.Record, error) {
				if failure {
					return nil, apperr.New("forbidden", "missing attachment scope")
				}
				return []xero.Record{{}, {}}, nil
			}}
			code, out, stderr := invoke([]string{"--config", path, group, "show", documentTestID}, "", func(string, config.OrgConfig) client { return fake })
			if code != 0 || stderr != "" || !strings.Contains(out, "yes") {
				t.Errorf("show %d %q %q", code, out, stderr)
			}
			if strings.Contains(out, "yes (2)") == failure {
				t.Errorf("count output %q", out)
			}
		}
	}
}

func TestDocumentShowJSONAndAttachmentLists(t *testing.T) {
	path := commandConfig(t)
	for _, group := range []string{"bank-transaction", "bank-transfer", "manual-journal", "contact"} {
		fake := &fakeClient{recordFn: func(string, string) (xero.Record, error) {
			return xero.Record{"Unknown": json.RawMessage(`true`), "Tracking": json.RawMessage(`[{"Name":"Region","Option":"North"}]`)}, nil
		}, attachmentsFn: func(string, string) ([]xero.Record, error) {
			t.Fatal("JSON show unnecessarily requested attachment count")
			return nil, nil
		}}
		code, out, stderr := invoke([]string{"--config", path, "--json", "--color", "always", group, "show", documentTestID}, "", func(string, config.OrgConfig) client { return fake })
		if code != 0 || stderr != "" || !json.Valid([]byte(out)) || strings.Count(out, "\n") != 1 || strings.Contains(out, "\x1b") || !strings.Contains(out, `"Unknown":true`) || !strings.Contains(out, `"Name":"Region"`) {
			t.Errorf("%s JSON = %d %q %q", group, code, out, stderr)
		}
	}
	for _, group := range []string{"bank-transaction", "manual-journal"} {
		fake := &fakeClient{attachmentsFn: func(string, string) ([]xero.Record, error) {
			return []xero.Record{{"FileName": json.RawMessage(`"bill.pdf"`), "Unknown": json.RawMessage(`true`)}}, nil
		}}
		code, out, stderr := invoke([]string{"--config", path, "--json", group, "attachments", documentTestID}, "", func(string, config.OrgConfig) client { return fake })
		if code != 0 || stderr != "" || !strings.Contains(out, `"complete":true`) || !strings.Contains(out, `"Unknown":true`) {
			t.Errorf("%s attachments = %d %q %q", group, code, out, stderr)
		}
	}
}

func TestAttachmentFailureLeavesNoFile(t *testing.T) {
	path := commandConfig(t)
	target := filepath.Join(t.TempDir(), "failed.pdf")
	fake := &fakeClient{attachmentFn: func(string, string, string) ([]byte, error) { return nil, apperr.New("api", "interrupted response") }}
	code, out, _ := invoke([]string{"--config", path, "bank-transaction", "attachment", documentTestID, "bill.pdf", "--output", target}, "", func(string, config.OrgConfig) client { return fake })
	if code != 1 || out != "" {
		t.Errorf("failure = %d %q", code, out)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("failed download created a file: %v", err)
	}
}
