package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func (o *options) report(command *cobra.Command, output reportOutput, title string, csvOutput bool) error {
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

func (o *options) object(command *cobra.Command, object any) error {
	if o.json {
		return render.JSON(command.OutOrStdout(), object)
	}
	// Struct order defines human field order; JSON preserves the full source object.
	value := reflect.ValueOf(object)
	typ := value.Type()
	var rows [][]string
	for i := range value.NumField() {
		if !value.Field(i).CanInterface() {
			continue
		}
		text := fmt.Sprint(value.Field(i).Interface())
		if typ.Field(i).Name == "UpdatedDateUTC" && text != "" {
			if date, err := xero.ParseDate(text); err == nil {
				text = date.Format(time.RFC3339Nano)
			}
		}
		rows = append(rows, []string{typ.Field(i).Name, text})
	}
	return render.Table(command.OutOrStdout(), nil, rows, o.color)
}

func (o *options) list(command *cobra.Command, identity xero.Identity, items any, header []string, rows [][]string) error {
	if o.json {
		return render.JSON(command.OutOrStdout(), struct {
			Org      xero.Identity `json:"org"`
			Complete bool          `json:"complete"`
			Items    any           `json:"items"`
		}{Org: identity, Complete: true, Items: items})
	}
	if _, err := fmt.Fprintf(command.OutOrStdout(), "Organisation: %s (%s)\n", identity.Name, identity.ID); err != nil {
		return err
	}
	return render.Table(command.OutOrStdout(), header, rows, o.color)
}

type apiError struct {
	body []byte
	err  error
}

func (e *apiError) Error() string { return e.err.Error() }
func (e *apiError) Unwrap() error { return e.err }
