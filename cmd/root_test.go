package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/config"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

type fakeClient struct {
	account       xero.Account
	accountErr    error
	accountID     string
	accounts      []xero.Account
	accountsQuery url.Values
	categories    []xero.TrackingCategory
	err           error
	identity      xero.Identity
	org           xero.Organisation
	report        xero.Report
	reportFn      func(string, url.Values) (xero.Report, error)
	request       xero.Request
	response      []byte
	status        xero.AuthStatus
}

func (f *fakeClient) Report(_ context.Context, name string, query url.Values) (xero.Report, error) {
	if f.reportFn != nil {
		return f.reportFn(name, query)
	}
	return f.report, f.err
}

func (f *fakeClient) Identity() xero.Identity                                 { return f.identity }
func (f *fakeClient) AuthStatus(context.Context) (xero.AuthStatus, error)     { return f.status, f.err }
func (f *fakeClient) Organisation(context.Context) (xero.Organisation, error) { return f.org, f.err }
func (f *fakeClient) Accounts(_ context.Context, query url.Values) ([]xero.Account, error) {
	f.accountsQuery = query
	return f.accounts, f.err
}
func (f *fakeClient) Account(_ context.Context, id string) (xero.Account, error) {
	f.accountID = id
	return f.account, f.accountErr
}
func (f *fakeClient) TrackingCategories(context.Context) ([]xero.TrackingCategory, error) {
	return f.categories, f.err
}
func (f *fakeClient) DoRequest(_ context.Context, request xero.Request) (int, http.Header, []byte, error) {
	f.request = request
	return 200, nil, f.response, f.err
}

func commandConfig(t *testing.T) string {
	t.Helper()
	t.Setenv("XERO_ORG", "")
	if err := os.Unsetenv("XERO_ORG"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`default_org = "acme"
[orgs.acme]
client_id = "00000000000000000000000000000000"
secret_file = "unread-secret"
organisation_id = "org-id"
[orgs.beta]
client_id = "11111111111111111111111111111111"
secret_file = "unread-secret"
organisation_id = "beta-id"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func invoke(args []string, in string, factory func(string, config.OrgConfig) client) (int, string, string) {
	var out, errOut bytes.Buffer
	code := execute(context.Background(), args, strings.NewReader(in), &out, &errOut, factory)
	return code, out.String(), errOut.String()
}

func walkCommands(root *cobra.Command, visit func(*cobra.Command)) {
	visit(root)
	for _, child := range root.Commands() {
		walkCommands(child, visit)
	}
}

func TestEveryApplicationCommandDeclaresPositionalGrammar(t *testing.T) {
	root, err := newRoot(&options{})
	if err != nil {
		t.Fatal(err)
	}
	walkCommands(root, func(command *cobra.Command) {
		if command != root && command.Args == nil {
			t.Errorf("%s has no positional grammar", command.CommandPath())
		}
		if command.HasSubCommands() && command.RunE == nil {
			t.Errorf("%s is not runnable", command.CommandPath())
		}
	})
}

func TestEveryApplicationCommandHasWrappedLongHelp(t *testing.T) {
	root, err := newRoot(&options{})
	if err != nil {
		t.Fatal(err)
	}
	walkCommands(root, func(command *cobra.Command) {
		if command.Long == "" {
			t.Errorf("%s has no Long", command.CommandPath())
		}
		for _, text := range []string{command.Long, command.Example} {
			for _, line := range strings.Split(text, "\n") {
				if len(line) > 80 {
					t.Errorf("%s: %d columns: %s", command.CommandPath(), len(line), line)
				}
			}
		}
	})
}

func TestExitCodesTopicPrintsSameHelpFromBothEntryPoints(t *testing.T) {
	code, direct, errOut := invoke([]string{"exit-codes"}, "", nil)
	otherCode, help, otherErr := invoke([]string{"help", "exit-codes"}, "", nil)
	if code != 0 || otherCode != 0 || errOut != "" || otherErr != "" || direct != help {
		t.Fatalf("topic differs: %d/%d stderr %q/%q", code, otherCode, errOut, otherErr)
	}
	for _, row := range []string{"  0  Success", "  1  Application error", "  2  Usage error"} {
		if !strings.Contains(direct, row) {
			t.Errorf("topic lacks %q", row)
		}
	}
}

func TestResourceCommandOutputs(t *testing.T) {
	path := commandConfig(t)
	fake := &fakeClient{identity: xero.Identity{Name: "acme", ID: "org-id"}, org: xero.Organisation{Name: "Example Ltd", OrganisationID: "org-id", BaseCurrency: "USD"}, accounts: []xero.Account{{Code: "200", Name: "Sales", Type: "REVENUE", TaxType: "NONE", Status: "ACTIVE"}}, categories: []xero.TrackingCategory{{Name: "Region", Status: "ACTIVE", Options: []xero.TrackingOption{{Name: "North", Status: "ACTIVE"}, {Name: "Former", Status: "ARCHIVED"}}}, {Name: "Old", Status: "ARCHIVED"}}}
	for _, tc := range []struct {
		args        []string
		human, json string
	}{
		{[]string{"org", "show"}, "Example Ltd", `"OrganisationID":"org-id"`},
		{[]string{"accounts", "list"}, "Sales", `"complete":true`},
		{[]string{"account", "show", "200"}, "Sales", `"Code":"200"`},
		{[]string{"tracking-categories", "list"}, "Region", `"items":[`},
		{[]string{"tracking-category", "show", "region"}, "Former", `"Status":"ARCHIVED"`},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(strings.Join(tc.args, " ")+fmt.Sprint(jsonMode), func(t *testing.T) {
				args := append([]string{"--config", path}, tc.args...)
				want := tc.human
				if jsonMode {
					args = append(args, "--json", "--color", "always")
					want = tc.json
				}
				code, out, errOut := invoke(args, "", func(string, config.OrgConfig) client { return fake })
				if code != 0 || errOut != "" || !strings.Contains(out, want) {
					t.Errorf("command = %d, stdout %q, stderr %q", code, out, errOut)
				}
				if jsonMode && (!json.Valid([]byte(out)) || strings.Count(out, "\n") != 1 || strings.Contains(out, "\x1b")) {
					t.Errorf("invalid machine output %q", out)
				}
			})
		}
	}
}

func TestAccountFilterAndResolution(t *testing.T) {
	path := commandConfig(t)
	fake := &fakeClient{accounts: []xero.Account{}}
	code, _, errOut := invoke([]string{"--config", path, "accounts", "list", "--bank", "--class", "asset", "--status", "all"}, "", func(string, config.OrgConfig) client { return fake })
	if code != 0 || fake.accountsQuery.Get("where") != `Type=="BANK" AND Class=="ASSET"` || fake.accountsQuery.Get("order") != "Code" {
		t.Errorf("filters = %v, code %d, %s", fake.accountsQuery, code, errOut)
	}
	id := "00000000-0000-0000-0000-000000000001"
	for _, tc := range []struct {
		name, operand                string
		accounts                     []xero.Account
		idResult                     xero.Account
		idErr                        error
		wantCode, requested, errCode string
	}{
		{"code first", id, []xero.Account{{Code: id, Name: "Code match"}}, xero.Account{}, nil, id, "", ""},
		{"ID fallback", id, nil, xero.Account{Code: "100"}, nil, "100", id, ""},
		{"name fallback", "sales", []xero.Account{{Code: "200", Name: "Sales"}}, xero.Account{}, nil, "200", "", ""},
		{"name after missing ID", id, []xero.Account{{Code: "300", Name: id}}, xero.Account{}, apperr.New("not_found", "missing"), "300", id, ""},
		{"ambiguous", "sales", []xero.Account{{Code: "200", Name: "Sales"}, {Code: "201", Name: "SALES"}}, xero.Account{}, nil, "", "", "invalid_argument"},
		{"missing", "none", nil, xero.Account{}, nil, "", "", "not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeClient{accounts: tc.accounts, account: tc.idResult, accountErr: tc.idErr}
			got, err := resolveAccount(context.Background(), fake, tc.operand)
			if got.Code != tc.wantCode || fake.accountID != tc.requested {
				t.Errorf("resolved code/ID = %q/%q, want %q/%q", got.Code, fake.accountID, tc.wantCode, tc.requested)
			}
			if tc.errCode == "" && err != nil || tc.errCode != "" && (err == nil || apperr.From(err).Code != tc.errCode) {
				t.Errorf("resolve error = %v, want %s", err, tc.errCode)
			}
		})
	}
}

func TestAuthStatusPrintsEveryOrganisationBeforeFailure(t *testing.T) {
	path := commandConfig(t)
	var names []string
	code, out, errOut := invoke([]string{"--config", path, "--json", "auth", "status"}, "", func(name string, _ config.OrgConfig) client {
		names = append(names, name)
		fake := &fakeClient{status: xero.AuthStatus{Name: name, TokenOK: true, OrganisationOK: true}}
		if name == "acme" {
			fake.err = apperr.New("forbidden", "missing settings scope")
		}
		return fake
	})
	if code != 1 || !reflect.DeepEqual(names, []string{"acme", "beta"}) || !strings.Contains(out, `"name":"beta"`) || !strings.Contains(errOut, `"code":"forbidden"`) {
		t.Errorf("status = %d / %q / %q, names %v", code, out, errOut, names)
	}
	var statuses []xero.AuthStatus
	if err := json.Unmarshal([]byte(out), &statuses); err != nil || len(statuses) != 2 {
		t.Errorf("status JSON = %v, %v", statuses, err)
	}
}

func TestUsageConflictsPrecedeConfigAndNoOrgPrecedesFactory(t *testing.T) {
	path := commandConfig(t)
	for _, args := range [][]string{{"config", "--json"}, {"tracking-categories", "list", "--all", "--archived"}, {"accounts", "list", "--bank", "--type", "revenue"}, {"api", "Accounts", "--param", "bad"}} {
		code, out, _ := invoke(append([]string{"--config", "missing-file"}, args...), "", func(string, config.OrgConfig) client { t.Error("factory called on usage error"); return nil })
		if code != 2 || out != "" {
			t.Errorf("usage %v = %d, %q", args, code, out)
		}
	}
	t.Setenv("XERO_ORG", "")
	code, out, errOut := invoke([]string{"--config", path, "accounts", "list"}, "", func(string, config.OrgConfig) client { t.Error("factory called without selection"); return nil })
	if code != 1 || out != "" || !strings.Contains(errOut, "no organisation selected") {
		t.Errorf("no org = %d, %q, %q", code, out, errOut)
	}
}

func TestAPIPassthroughAndErrorStreams(t *testing.T) {
	path := commandConfig(t)
	fake := &fakeClient{response: []byte("{ \"untouched\": 1 }\n")}
	args := []string{"--config", path, "--json", "api", "Accounts", "--method", "PUT", "--param", "key=a=b", "--param", "key=two", "--input", "-", "--modified-since", "2026-01-02T03:04:05-08:00"}
	code, out, errOut := invoke(args, `{"body":1}`, func(string, config.OrgConfig) client { return fake })
	if code != 0 || out != string(fake.response) || errOut != "" {
		t.Errorf("api streams = %d/%q/%q", code, out, errOut)
	}
	r := fake.request
	if r.Method != "PUT" || r.Path != "Accounts" || string(r.Body) != `{"body":1}` || !reflect.DeepEqual(r.Query["key"], []string{"a=b", "two"}) || r.Headers.Get("If-Modified-Since") != "2026-01-02T11:04:05" {
		t.Errorf("forwarded request = %#v", r)
	}
	fake.err = apperr.New("forbidden", "missing role")
	fake.response = []byte(`{"Detail":"missing role"}`)
	code, out, errOut = invoke(args, "{}", func(string, config.OrgConfig) client { return fake })
	if code != 1 || out != "" || !strings.Contains(errOut, `"code":"forbidden"`) || !strings.HasSuffix(errOut, string(fake.response)+"\n") {
		t.Errorf("api error = %d/%q/%q", code, out, errOut)
	}
}

func TestOfflineOrgsMarksSelection(t *testing.T) {
	path := commandConfig(t)
	for _, name := range []string{"acme", "beta"} {
		code, out, errOut := invoke([]string{"--config", path, "--org", name, "--json", "orgs"}, "", func(string, config.OrgConfig) client { t.Error("orgs called factory"); return nil })
		var entries []struct {
			Name      string
			Effective bool
		}
		if err := json.Unmarshal([]byte(out), &entries); err != nil {
			t.Fatal(err)
		}
		if code != 0 || errOut != "" || len(entries) != 2 {
			t.Fatalf("orgs = %d/%q/%q", code, out, errOut)
		}
		for _, entry := range entries {
			if entry.Effective != (entry.Name == name) {
				t.Errorf("effective marker = %#v", entry)
			}
		}
	}
}

func TestStartupCreatesNoFilesOrRequests(t *testing.T) {
	buildDir := t.TempDir()
	binary := filepath.Join(buildDir, "xero")
	build := exec.Command("go", "build", "-o", binary, "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build startup binary: %v\n%s", err, output)
	}
	var calls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(502) }))
	defer proxy.Close()
	for _, args := range [][]string{{"--help"}, {"--version"}, {"config"}, {"orgs"}, {"unknown-command"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			home := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, args...)
			command.Dir = home
			command.Env = []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "HTTP_PROXY=" + proxy.URL, "HTTPS_PROXY=" + proxy.URL}
			output, err := command.CombinedOutput()
			if args[0] == "unknown-command" {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 2 {
					t.Errorf("rejected startup = %v", err)
				}
			} else if err != nil {
				t.Errorf("startup = %v: %s", err, output)
			}
			entries, err := os.ReadDir(home)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("startup created %v", entries)
			}
		})
	}
	if calls.Load() != 0 {
		t.Errorf("startup made %d network requests", calls.Load())
	}
}

type unreadableInput struct{ t *testing.T }

func (r unreadableInput) Read([]byte) (int, error) {
	r.t.Error("stdin read before organisation identity was configured")
	return 0, fmt.Errorf("unexpected stdin read")
}

func TestMissingOrganisationIDPrecedesAPIInput(t *testing.T) {
	path := commandConfig(t)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content = bytes.ReplaceAll(content, []byte(`organisation_id = "org-id"`), []byte(`organisation_id = ""`))
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := execute(context.Background(), []string{"--config", path, "api", "Accounts", "--input", "-"}, unreadableInput{t}, &out, &errOut, func(string, config.OrgConfig) client {
		t.Error("client constructed before organisation ID was configured")
		return nil
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "organisation_id is not set") {
		t.Errorf("missing ID = %d/%q/%q", code, out.String(), errOut.String())
	}
}
