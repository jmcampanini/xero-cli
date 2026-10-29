package cli

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/outputfile"
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

type collectionOptions struct {
	csv        bool
	jsonl      bool
	noHeader   bool
	outputFile string
	overwrite  bool
}

type collectionResult struct {
	Org      xero.Identity `json:"org"`
	Complete bool          `json:"complete"`
	Items    any           `json:"items"`
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

func (o *options) list(command *cobra.Command, output *collectionOptions, identity xero.Identity, items any, header []string, rows [][]string) error {
	if o.json || output.jsonl || output.csv {
		return output.write(command, collectionResult{Org: identity, Complete: true, Items: items})
	}
	if _, err := fmt.Fprintf(command.OutOrStdout(), "Organisation: %s (%s)\n", identity.Name, identity.ID); err != nil {
		return err
	}
	return render.Table(command.OutOrStdout(), header, rows, o.color)
}

func (c *collectionOptions) flags(command *cobra.Command) {
	flags := command.Flags()
	flags.BoolVar(&c.jsonl, "jsonl", false, "write one JSON object per record")
	flags.BoolVar(&c.csv, "csv", false, "write native fields as CSV")
	flags.BoolVar(&c.noHeader, "no-header", false, "omit the CSV header")
	flags.StringVar(&c.outputFile, "output-file", "", "write machine output to this file")
	flags.BoolVar(&c.overwrite, "overwrite", false, "replace an existing --output-file")
}

func (c *collectionOptions) validate(command *cobra.Command, jsonOutput bool) error {
	formats := 0
	for _, selected := range []bool{jsonOutput, c.jsonl, c.csv} {
		if selected {
			formats++
		}
	}
	if formats > 1 {
		return apperr.Usage("--json, --jsonl and --csv cannot be combined")
	}
	if c.noHeader && !c.csv {
		return apperr.Usage("--no-header requires --csv")
	}
	if command.Flags().Changed("output-file") {
		if c.outputFile == "" {
			return apperr.Usage("--output-file must not be empty")
		}
		if formats == 0 {
			return apperr.Usage("--output-file requires --json, --jsonl or --csv")
		}
	} else if c.overwrite {
		return apperr.Usage("--overwrite requires --output-file")
	}
	return nil
}

func (c *collectionOptions) write(command *cobra.Command, result collectionResult) error {
	var payload bytes.Buffer
	var err error
	switch {
	case c.jsonl:
		err = render.JSONL(&payload, result.Items)
	case c.csv:
		err = render.CSV(&payload, result.Items, c.noHeader)
	default:
		err = render.JSON(&payload, result)
	}
	if err != nil {
		return err
	}
	if c.outputFile != "" {
		return outputfile.Write(command.Context(), c.outputFile, payload.Bytes(), c.overwrite)
	}
	_, err = command.OutOrStdout().Write(payload.Bytes())
	return err
}
