package render

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/xero"
)

func TestReportTableSectionsPrecisionAndColor(t *testing.T) {
	rows := []xero.ReportRow{
		{Section: []string{}, Label: "Balance", Kind: "heading", Values: []string{"", ""}},
		{Section: []string{"Assets", "Current"}, AccountCode: "100", Label: "Cash", Kind: "row", Values: []string{"12345678901234567890.1234", ""}},
		{Section: []string{"Assets", "Current"}, AccountCode: "9", Label: "Petty cash", Kind: "row", Values: []string{"1.00", ""}},
		{Section: []string{"Assets", "Current"}, Label: "Total current", Kind: "summary", Values: []string{"-1234.00", "0.00"}},
	}
	var out bytes.Buffer
	if err := ReportTable(&out, []string{"Now", "Before"}, rows, "always"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"\nBalance\nAssets\nCurrent\n", "  Code  Account", "  100   Cash", "  9     Petty cash", "12,345,678,901,234,567,890.1234", "-1,234.00", "\x1b[1m        Total current"} {
		if !strings.Contains(text, want) {
			t.Errorf("table %q missing %q", text, want)
		}
	}
	if strings.Count(text, "Current\n") != 1 {
		t.Errorf("section repeated in %q", text)
	}
	out.Reset()
	if err := ReportTable(&out, []string{"Now", "Before"}, rows, "auto"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Error("redirected auto output contains ANSI")
	}
}

func TestReportCSVQuotesLabelsAndKeepsBlanks(t *testing.T) {
	rows := []xero.ReportRow{{Section: []string{"Assets", "Current"}, AccountCode: "100", Label: "Cash, \"bank\"", Values: []string{"-1234.5678", ""}}}
	var out bytes.Buffer
	if err := ReportCSV(&out, []string{"Current", "Prior"}, rows); err != nil {
		t.Fatal(err)
	}
	got, err := csv.NewReader(&out).ReadAll()
	want := [][]string{{"section", "label", "account_code", "Current", "Prior"}, {"Assets > Current", "Cash, \"bank\"", "100", "-1234.5678", ""}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("CSV = %v, %v; want %v", got, err, want)
	}
}
