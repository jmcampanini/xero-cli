package render

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/jmcampanini/xero-cli/internal/xero"
)

// ReportTable renders native labels and values with section paths and bold totals.
// Account codes share one left-aligned column; heading rows print as titles.
func ReportTable(w io.Writer, columns []string, rows []xero.ReportRow, color string) error {
	widths := make([]int, len(columns)+1)
	widths[0] = utf8.RuneCountInString("  Code  Account")
	for i, column := range columns {
		widths[i+1] = utf8.RuneCountInString(cellSanitizer.Replace(column))
	}
	codeWidth := len("Code")
	for _, row := range rows {
		codeWidth = max(codeWidth, utf8.RuneCountInString(cellSanitizer.Replace(row.AccountCode)))
	}
	labels := make([]string, len(rows))
	values := make([][]string, len(rows))
	for i, row := range rows {
		labels[i] = fmt.Sprintf("  %-*s  %s", codeWidth, cellSanitizer.Replace(row.AccountCode), cellSanitizer.Replace(row.Label))
		if row.Kind != "heading" {
			widths[0] = max(widths[0], utf8.RuneCountInString(labels[i]))
		}
		if len(row.Values) != len(columns) {
			return fmt.Errorf("report row %q has inconsistent columns", row.Label)
		}
		for j, value := range row.Values {
			formatted := cellSanitizer.Replace(groupDecimal(value))
			values[i] = append(values[i], formatted)
			widths[j+1] = max(widths[j+1], utf8.RuneCountInString(formatted))
		}
	}
	var buf strings.Builder
	writeLine := func(label string, cells []string, bold bool) {
		if bold {
			buf.WriteString("\x1b[1m")
		}
		fmt.Fprintf(&buf, "%-*s", widths[0], label)
		for i, cell := range cells {
			fmt.Fprintf(&buf, "  %*s", widths[i+1], cellSanitizer.Replace(cell))
		}
		if bold {
			buf.WriteString("\x1b[0m")
		}
		buf.WriteByte('\n')
	}
	writeLine(fmt.Sprintf("  %-*s  Account", codeWidth, "Code"), columns, ColorEnabled(w, color))
	var section []string
	for i, row := range rows {
		common := 0
		for common < len(section) && common < len(row.Section) && section[common] == row.Section[common] {
			common++
		}
		for _, title := range row.Section[common:] {
			buf.WriteString(cellSanitizer.Replace(title) + "\n")
		}
		section = row.Section
		if row.Kind == "heading" {
			buf.WriteString(cellSanitizer.Replace(row.Label) + "\n")
			continue
		}
		writeLine(labels[i], values[i], row.Kind == "summary" && ColorEnabled(w, color))
	}
	_, err := io.WriteString(w, buf.String())
	return err
}

// ReportCSV writes a plain CSV table with unmodified decimal strings.
func ReportCSV(w io.Writer, columns []string, rows []xero.ReportRow) error {
	var buf bytes.Buffer
	csvWriter := csv.NewWriter(&buf)
	if err := csvWriter.Write(append([]string{"section", "label", "account_code"}, columns...)); err != nil {
		return err
	}
	for _, row := range rows {
		if len(row.Values) != len(columns) {
			return fmt.Errorf("report row %q has inconsistent columns", row.Label)
		}
		if err := csvWriter.Write(append([]string{strings.Join(row.Section, " > "), row.Label, row.AccountCode}, row.Values...)); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

func groupDecimal(value string) string {
	integer, fraction, point := strings.Cut(value, ".")
	sign := ""
	if strings.HasPrefix(integer, "-") || strings.HasPrefix(integer, "+") {
		sign, integer = integer[:1], integer[1:]
	}
	if integer == "" || point && fraction == "" {
		return value
	}
	for _, digit := range integer + fraction {
		if digit < '0' || digit > '9' {
			return value
		}
	}
	var grouped strings.Builder
	grouped.WriteString(sign)
	for i, digit := range integer {
		if i > 0 && (len(integer)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	if point {
		grouped.WriteByte('.')
		grouped.WriteString(fraction)
	}
	return grouped.String()
}
