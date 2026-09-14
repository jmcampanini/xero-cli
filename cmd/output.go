package cmd

import (
	"fmt"
	"reflect"
	"time"

	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

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
