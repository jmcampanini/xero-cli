package xero

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"golang.org/x/time/rate"
)

func jwt(scopes []string) string {
	payload, _ := json.Marshal(map[string]any{"exp": time.Now().Add(30 * time.Minute).Unix(), "scope": scopes})
	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func testClient(t *testing.T, handlers map[string]http.HandlerFunc) *Client {
	t.Helper()
	defaults := map[string]http.HandlerFunc{
		"/token": func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": jwt([]string{"accounting.settings.read"}), "token_type": "Bearer", "expires_in": 1800})
		},
		"/connections": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `[{"tenantId":"org-id","tenantName":"Example Ltd"}]`)
		},
	}
	for path, handler := range handlers {
		defaults[path] = handler
	}
	mux := http.NewServeMux()
	for path, handler := range defaults {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			handler(w, r)
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("private-test-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	return New(Options{BaseURL: server.URL + "/api/", Budget: &Budget{limiter: rate.NewLimiter(rate.Inf, 1), slots: make(chan struct{}, 5)}, ClientID: strings.Repeat("a", 32), ConnectionsURL: server.URL + "/connections", HTTPClient: server.Client(), Name: "acme", OrganisationID: "org-id", SecretFile: secret, TokenURL: server.URL + "/token", Version: "test-version"})
}

func TestClientCredentialsAndAccountingHeaders(t *testing.T) {
	for _, scopes := range [][]string{nil, {"accounting.settings.read", "accounting.contacts.read"}} {
		t.Run(strings.Join(scopes, " "), func(t *testing.T) {
			var tokens, connections, accounts atomic.Int32
			c := testClient(t, map[string]http.HandlerFunc{
				"/token": func(w http.ResponseWriter, r *http.Request) {
					tokens.Add(1)
					id, secret, ok := r.BasicAuth()
					if !ok || id != strings.Repeat("a", 32) || secret != "private-test-secret" {
						t.Error("token request did not use the configured Basic credentials")
					}
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					want := url.Values{"grant_type": {"client_credentials"}}
					if len(scopes) > 0 {
						want.Set("scope", strings.Join(scopes, " "))
					}
					if !reflect.DeepEqual(r.PostForm, want) {
						t.Errorf("token form = %v, want %v", r.PostForm, want)
					}
					_, _ = io.WriteString(w, `{"access_token":"token-one","token_type":"Bearer","expires_in":1800}`)
				},
				"/connections": func(w http.ResponseWriter, r *http.Request) {
					connections.Add(1)
					if r.Header.Get("Authorization") != "Bearer token-one" {
						t.Error("connections missing bearer token")
					}
					_, _ = io.WriteString(w, `[{"tenantId":"org-id","tenantName":"Example"}]`)
				},
				"/api/Accounts": func(w http.ResponseWriter, r *http.Request) {
					accounts.Add(1)
					for key, want := range map[string]string{"Authorization": "Bearer token-one", "xero-tenant-id": "org-id", "Accept": "application/json", "User-Agent": "xero-cli/test-version", "Content-Type": "application/json", "If-Modified-Since": "2026-01-02T03:04:05"} {
						if got := r.Header.Get(key); got != want {
							t.Errorf("header %s = %q, want %q", key, got, want)
						}
					}
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != `{"test":1}` {
						t.Errorf("request body = %q, %v", body, err)
					}
					if r.Method != "POST" || r.URL.Query().Get("where") != "Type==\"BANK\"" {
						t.Errorf("request = %s %s", r.Method, r.URL.String())
					}
					w.Header().Set("X-DayLimit-Remaining", "987")
					_, _ = io.WriteString(w, `{"Accounts":[]}`)
				},
			})
			c.options.Scopes = scopes
			for range 2 {
				_, _, _, err := c.DoRequest(context.Background(), Request{Method: "POST", Path: "Accounts", Query: url.Values{"where": {`Type=="BANK"`}}, Body: []byte(`{"test":1}`), Headers: http.Header{"If-Modified-Since": {"2026-01-02T03:04:05"}}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if tokens.Load() != 1 || connections.Load() != 1 || accounts.Load() != 2 {
				t.Errorf("calls token/connections/accounts = %d/%d/%d", tokens.Load(), connections.Load(), accounts.Load())
			}
			if got := c.Limits().Get("X-DayLimit-Remaining"); got != "987" {
				t.Errorf("day remaining = %q", got)
			}
		})
	}
}

func TestIdentityFailureStopsAccounting(t *testing.T) {
	for _, tc := range []struct{ name, id, code string }{{"unset", "", "invalid_argument"}, {"mismatch", "wrong-id", "forbidden"}} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			c := testClient(t, map[string]http.HandlerFunc{"/api/Accounts": func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, `{"Accounts":[]}`)
			}})
			c.options.OrganisationID = tc.id
			_, err := c.Accounts(context.Background(), nil)
			if err == nil || apperr.From(err).Code != tc.code {
				t.Errorf("Accounts error = %v, want %s", err, tc.code)
			}
			if calls.Load() != 0 {
				t.Error("accounting was called before identity verified")
			}
			status, err := c.AuthStatus(context.Background())
			if tc.id == "" {
				if err != nil || status.OrganisationID != "org-id" {
					t.Errorf("discovery = %#v, %v", status, err)
				}
			} else if err == nil || status.OrganisationOK {
				t.Errorf("mismatch status = %#v, %v", status, err)
			}
		})
	}
}

func TestTokenErrorsAreActionableAndRedacted(t *testing.T) {
	for _, code := range []string{"invalid_client", "invalid_scope"} {
		t.Run(code, func(t *testing.T) {
			c := testClient(t, map[string]http.HandlerFunc{"/token": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(400)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": "private-test-secret"})
			}})
			_, err := c.Accounts(context.Background(), nil)
			if err == nil || apperr.From(err).Code != "unauthenticated" || !strings.Contains(err.Error(), code) || strings.Contains(err.Error(), "private-test-secret") {
				t.Errorf("token error = %v", err)
			}
		})
	}
}

func TestUnauthorizedRefetchesAndReplaysBodyOnce(t *testing.T) {
	for _, always := range []bool{false, true} {
		t.Run(fmt.Sprint(always), func(t *testing.T) {
			var tokens, calls atomic.Int32
			c := testClient(t, map[string]http.HandlerFunc{
				"/token": func(w http.ResponseWriter, _ *http.Request) {
					n := tokens.Add(1)
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": fmt.Sprint("token-", n), "token_type": "Bearer", "expires_in": 1800})
				},
				"/api/Accounts": func(w http.ResponseWriter, r *http.Request) {
					n := calls.Add(1)
					body, _ := io.ReadAll(r.Body)
					if string(body) != "payload" {
						t.Error("retry lost request body")
					}
					if n == 1 || always {
						w.WriteHeader(401)
						_, _ = io.WriteString(w, `{"Detail":"expired"}`)
						return
					}
					if r.Header.Get("Authorization") != "Bearer token-2" {
						t.Error("retry reused stale token")
					}
					_, _ = io.WriteString(w, `{}`)
				},
			})
			_, _, _, err := c.Do(context.Background(), "POST", "Accounts", nil, []byte("payload"))
			if always {
				if err == nil || apperr.From(err).Code != "unauthenticated" {
					t.Errorf("second 401 = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tokens.Load() != 2 || calls.Load() != 2 {
				t.Errorf("tokens/calls = %d/%d", tokens.Load(), calls.Load())
			}
		})
	}
}

func TestRateLimitRetriesOnce(t *testing.T) {
	for _, always := range []bool{false, true} {
		t.Run(fmt.Sprint(always), func(t *testing.T) {
			var calls atomic.Int32
			c := testClient(t, map[string]http.HandlerFunc{"/api/Accounts": func(w http.ResponseWriter, _ *http.Request) {
				n := calls.Add(1)
				w.Header().Set("Retry-After", "0")
				if n == 1 || always {
					w.WriteHeader(429)
				}
				_, _ = io.WriteString(w, `{}`)
			}})
			_, _, _, err := c.Do(context.Background(), "GET", "Accounts", nil, nil)
			if calls.Load() != 2 {
				t.Errorf("requests = %d, want 2", calls.Load())
			}
			if always {
				if err == nil || apperr.From(err).Code != "rate_limited" || !strings.Contains(err.Error(), "0s") {
					t.Errorf("rate error = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
	if got := retryDelay(http.Header{"Retry-After": {"120"}}); got != 60*time.Second {
		t.Errorf("retry cap = %v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait(ctx, time.Minute); err == nil {
		t.Error("retry wait ignored cancellation")
	}
}

func TestResponseErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		status           int
		body, code, part string
	}{
		{400, `{"Type":"ValidationException","Elements":[{"ValidationErrors":[{"Message":"first"},{"Message":"second"}]}]}`, "invalid_argument", "first; second"},
		{400, `{"Type":"ValidationException","Message":"fallback"}`, "invalid_argument", "fallback"},
		{401, `{}`, "unauthenticated", "401"},
		{403, `{"Detail":"missing role"}`, "forbidden", "missing role"},
		{404, `{}`, "not_found", "404"},
		{429, `{}`, "rate_limited", "retry after"},
		{500, `{"Message":"server down"}`, "api", "server down"},
		{502, `{"Detail":"upstream"}`, "api", "upstream"},
		{503, `{"Title":"unavailable"}`, "api", "unavailable"},
		{409, `{"Message":"other"}`, "api", "other"},
		{502, strings.Repeat("x", 300), "api", strings.Repeat("x", 200)},
	} {
		t.Run(fmt.Sprint(tc.status, "-", tc.part[:min(len(tc.part), 12)]), func(t *testing.T) {
			err := responseError(tc.status, http.Header{}, []byte(tc.body))
			if apperr.From(err).Code != tc.code || !strings.Contains(err.Error(), tc.part) {
				t.Errorf("responseError = %v, want %s / %s", err, tc.code, tc.part)
			}
			if strings.Contains(err.Error(), strings.Repeat("x", 201)) {
				t.Error("non-JSON error exceeded 200 bytes")
			}
		})
	}
}

func TestAuthStatusScopePolicy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scopes []string
		code   string
	}{
		{"this milestone", []string{"accounting.settings.read"}, ""},
		{"write includes read", []string{"accounting.settings"}, ""},
		{"missing settings", []string{"accounting.contacts.read"}, "forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(t, map[string]http.HandlerFunc{"/token": func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": jwt(tc.scopes), "expires_in": 1800, "token_type": "Bearer"})
			}})
			status, err := c.AuthStatus(context.Background())
			if !status.TokenOK || !status.OrganisationOK {
				t.Errorf("status = %#v", status)
			}
			if tc.code == "" && err != nil || tc.code != "" && (err == nil || apperr.From(err).Code != tc.code) {
				t.Errorf("AuthStatus error = %v, want %q", err, tc.code)
			}
			raw, _ := json.Marshal(status)
			if strings.Contains(string(raw), "signature") || strings.Contains(string(raw), "private-test-secret") {
				t.Error("status leaked credentials")
			}
		})
	}
	capabilities := Capabilities([]string{"accounting.banktransactions.read", "accounting.manualjournals.read", "accounting.reports.profitandloss.read"})
	if !capabilities["transactions"] || !capabilities["reports"] || capabilities["transactions writes"] {
		t.Errorf("granular scope mapping = %v", capabilities)
	}
}

func TestAPIPathCannotEscapeAccountingOrigin(t *testing.T) {
	c := testClient(t, nil)
	for _, path := range []string{"https://example.com/steal", "//example.com/steal", "../connections", "%2e%2e/connections", "/connections", "Accounts?where=anything", "Accounts#fragment"} {
		_, _, _, err := c.Do(context.Background(), "GET", path, nil, nil)
		if err == nil || apperr.From(err).Code != "invalid_argument" {
			t.Errorf("Do(%q) error = %v", path, err)
		}
	}
}

func TestDateAndNativeObjectPreservation(t *testing.T) {
	date, err := ParseDate("/Date(1573755038314+0000)/")
	if err != nil || date.Format(time.RFC3339Nano) != "2019-11-14T18:10:38.314Z" {
		t.Errorf("ParseDate = %v, %v", date, err)
	}
	for _, value := range []string{"bad", "/Date(no)/"} {
		if _, err := ParseDate(value); err == nil {
			t.Errorf("invalid date %q accepted", value)
		}
	}
	raw := []byte(`{"AccountID":"id","Code":"200","Name":"Sales","UpdatedDateUTC":"/Date(1573755038314+1300)/","Date":"/Date(1573689600000-0800)/","UnknownField":9007199254740993}`)
	var account Account
	if err := json.Unmarshal(raw, &account); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`"UpdatedDateUTC":"2019-11-14T18:10:38.314Z"`, `"Date":"2019-11-14"`, `"UnknownField":9007199254740993`} {
		if !strings.Contains(string(encoded), part) {
			t.Errorf("encoded object missing %s: %s", part, encoded)
		}
	}
}

func TestPagingAcrossThreePages(t *testing.T) {
	var pages []string
	do := func(_ context.Context, r Request) (int, http.Header, []byte, error) {
		pages = append(pages, r.Query.Get("page"))
		if r.Query.Get("pageSize") != "100" || r.Query.Get("where") != "filter" {
			t.Errorf("paging query = %v", r.Query)
		}
		return 200, nil, []byte(fmt.Sprintf(`{"pagination":{"page":%s,"pageSize":100,"pageCount":3,"itemCount":3},"Items":[{"ID":%s}]}`, r.Query.Get("page"), r.Query.Get("page"))), nil
	}
	query := url.Values{"where": {"filter"}, "page": {"99"}}
	_, _, body, err := All(context.Background(), Request{Path: "Items", Method: "GET", Query: query}, do)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pages, []string{"1", "2", "3"}) || query.Get("page") != "99" {
		t.Errorf("pages/query = %v / %v", pages, query)
	}
	var result struct{ Items []struct{ ID int } }
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 3 || result.Items[2].ID != 3 {
		t.Errorf("combined response = %s", body)
	}
	for _, body := range []string{`{"Items":[]}`, `{"pagination":{"page":1,"pageCount":1},"One":[],"Two":[]}`} {
		_, _, _, err := All(context.Background(), Request{}, func(context.Context, Request) (int, http.Header, []byte, error) { return 200, nil, []byte(body), nil })
		if err == nil || apperr.From(err).Code != "invalid_argument" {
			t.Errorf("invalid paging = %v", err)
		}
	}
}
