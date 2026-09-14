package xero

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestRecordedResourcesPreserveSourceFields(t *testing.T) {
	// Recorded September 14, 2026. Names, IDs, codes and dates are anonymized;
	// optional fields retain their response shape to detect accidental data loss.
	accounts, err := os.ReadFile("testdata/accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	tracking, err := os.ReadFile("testdata/tracking.json")
	if err != nil {
		t.Fatal(err)
	}
	c := testClient(t, map[string]http.HandlerFunc{
		"/api/Accounts": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(accounts) },
		"/api/Accounts/00000000-0000-0000-0000-000000000001": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(accounts) },
		"/api/TrackingCategories": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("includeArchived") != "true" {
				t.Error("archived tracking categories were excluded")
			}
			_, _ = w.Write(tracking)
		},
		"/api/Organisation": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"Organisations":[{"OrganisationID":"org-id","Name":"Example Ltd","UnknownSetting":true}]}`))
		},
	})

	items, err := c.Accounts(context.Background(), nil)
	if err != nil || len(items) != 1 {
		t.Fatalf("Accounts = %v, %v", items, err)
	}
	account, err := c.Account(context.Background(), items[0].AccountID)
	if err != nil || account.Code != "200" {
		t.Fatalf("Account = %v, %v", account, err)
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"ReportingName":""`, `"HasAttachments":false`, `"AddToWatchlist":true`, `"UpdatedDateUTC":"2019-11-14T18:10:38.314Z"`} {
		if !strings.Contains(string(encoded), field) {
			t.Errorf("account omitted %s: %s", field, encoded)
		}
	}

	categories, err := c.TrackingCategories(context.Background())
	if err != nil || len(categories) != 1 || len(categories[0].Options) != 8 {
		t.Fatalf("TrackingCategories = %v, %v", categories, err)
	}
	encoded, err = json.Marshal(categories)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"HasValidationErrors":false`) {
		t.Error("tracking output dropped optional source fields")
	}

	org, err := c.Organisation(context.Background())
	if err != nil || org.Name != "Example Ltd" {
		t.Fatalf("Organisation = %v, %v", org, err)
	}
	encoded, err = json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"UnknownSetting":true`) {
		t.Error("organisation output dropped a source field")
	}
}
