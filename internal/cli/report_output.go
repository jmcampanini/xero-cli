package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/reporting"
	"github.com/spf13/cobra"
)

func (o *options) report(command *cobra.Command, output reporting.Output, title string, csvOutput bool) error {
	if o.json {
		if output.Report == "BankSummary" {
			raw, err := json.Marshal(output)
			if err != nil {
				return err
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				return err
			}
			delete(fields, "filters")
			delete(fields, "by")
			return render.JSON(command.OutOrStdout(), fields)
		}
		return render.JSON(command.OutOrStdout(), output)
	}
	period := output.Date
	if period == "" {
		period = output.From + " to " + output.To
	}
	parts := []string{output.Org.Name, title, period}
	if output.Basis != "" {
		parts = append(parts, output.Basis)
	}
	parts = append(parts, output.Currency)
	context := strings.Join(parts, " · ")
	if output.Filters != nil {
		for _, filter := range output.Filters.Tracking {
			context += "\nfilter: " + filter.Category + "=" + filter.Option
		}
	}
	if output.By != nil {
		context += "\nby: " + output.By.Category
		if output.By.Note != "" {
			context += " (" + output.By.Note + ")"
		}
	}
	context = strings.NewReplacer("\x1b", "", "\r", " ", "\t", " ").Replace(context)
	var table bytes.Buffer
	if csvOutput {
		if err := render.ReportCSV(&table, output.Columns, output.Rows); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(command.ErrOrStderr(), context); err != nil {
			return err
		}
	} else {
		// Resolve color against stdout now; the table is assembled in a buffer.
		color := "never"
		if render.ColorEnabled(command.OutOrStdout(), o.color) {
			color = "always"
		}
		fmt.Fprintln(&table, context+"\n")
		if err := render.ReportTable(&table, output.Columns, output.Rows, color); err != nil {
			return err
		}
	}
	_, err := command.OutOrStdout().Write(table.Bytes())
	return err
}
