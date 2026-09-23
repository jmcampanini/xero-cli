package render

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/xero"
)

// Transactions writes the assembled account view, with CSV context on stderr.
func Transactions(out, diagnostics io.Writer, result xero.AccountTransactions, csvOutput bool, color string) error {
	var payload, context bytes.Buffer
	balance := result.OpeningBalance != ""
	fmt.Fprintf(&context, "%s · %s %s · %s to %s · cash · %s\n", cellSanitizer.Replace(result.Org.Name), cellSanitizer.Replace(result.Account.Code), cellSanitizer.Replace(result.Account.Name), result.From, result.To, result.Currency)
	for _, filter := range result.Filters.Tracking {
		fmt.Fprintf(&context, "filter: %s=%s (matching movements only)\n", cellSanitizer.Replace(filter.Category), cellSanitizer.Replace(filter.Option))
	}
	if len(result.Filters.Tracking) > 0 && result.Account.Type == "BANK" {
		fmt.Fprintln(&context, "bank rows retain whole-document amounts, not tracking allocations")
	}
	if balance {
		fmt.Fprintf(&context, "opening balance %s (%s)\n", groupDecimal(result.OpeningBalance), strings.ReplaceAll(result.OpeningPeriod, "_", " "))
	}
	if csvOutput {
		columns := []string{"date", "source_type", "source_subtype", "source_id", "contact", "description", "reference", "debit", "credit"}
		if balance {
			columns = append(columns, "balance")
		}
		columns = append(columns, "tracking", "status")
		writer := csv.NewWriter(&payload)
		if err := writer.Write(columns); err != nil {
			return err
		}
		for _, row := range result.Rows {
			values := []string{row.Date, row.SourceType, row.SourceSubtype, row.SourceID, row.Contact, row.Description, row.Reference, row.Debit, row.Credit}
			if balance {
				values = append(values, row.Balance)
			}
			values = append(values, trackingLabel(row.Tracking), row.Status)
			if err := writer.Write(values); err != nil {
				return err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
	} else {
		payload.Write(context.Bytes())
		payload.WriteByte('\n')
		columns := []string{"DATE", "SOURCE", "CONTACT / NARRATION", "DESCRIPTION", "REFERENCE", "DEBIT", "CREDIT"}
		if balance {
			columns = append(columns, "BALANCE")
		}
		columns = append(columns, "TRACKING", "STATUS")
		var rows [][]string
		for _, row := range result.Rows {
			id := row.SourceID
			if len(id) > 8 {
				id = id[:8] + "…"
			}
			values := []string{row.Date, strings.ToLower(row.SourceSubtype) + " " + id, row.Contact, row.Description, row.Reference, groupDecimal(row.Debit), groupDecimal(row.Credit)}
			if balance {
				values = append(values, groupDecimal(row.Balance))
			}
			values = append(values, trackingLabel(row.Tracking), row.Status)
			rows = append(rows, values)
		}
		if err := Table(&payload, columns, rows, bufferedColor(out, color)); err != nil {
			return err
		}
	}
	var footer bytes.Buffer
	fmt.Fprintf(&footer, "net movement %s\n", groupDecimal(result.NetMovement))
	if balance {
		fmt.Fprintf(&footer, "reconstructed closing balance %s", groupDecimal(result.ClosingBalance))
		if result.TrialBalance != "" {
			fmt.Fprintf(&footer, " · cash trial balance %s · unexplained %s", groupDecimal(result.TrialBalance), groupDecimal(result.Unexplained))
		}
		footer.WriteByte('\n')
	}
	fmt.Fprintf(&footer, "incomplete coverage: %s\n", result.Gap)
	if csvOutput {
		context.Write(footer.Bytes())
		if _, err := diagnostics.Write(context.Bytes()); err != nil {
			return err
		}
	} else {
		payload.Write(footer.Bytes())
	}
	_, err := out.Write(payload.Bytes())
	return err
}
