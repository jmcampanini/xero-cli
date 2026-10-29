package cli

import (
	"fmt"
	"strconv"

	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func (o *options) records(command *cobra.Command, output *collectionOptions, api client, resource string, query xero.ListQuery) error {
	result, err := api.Records(command.Context(), resource, query)
	if err != nil {
		return err
	}
	if result.Items == nil {
		result.Items = []xero.Record{}
	}
	if resource == "BankTransfers" && command.Flags().Changed("page") {
		if _, err := fmt.Fprintln(command.ErrOrStderr(), "BankTransfers is not paginated; --page and --page-size are ignored"); err != nil {
			return err
		}
	} else if query.Page != 0 {
		if _, err := fmt.Fprintf(command.ErrOrStderr(), "page %d of %d (%d items)\n", result.Page, result.PageCount, result.ItemCount); err != nil {
			return err
		}
	}
	if o.json || output.jsonl || output.csv {
		return output.write(command, collectionResult{Org: api.Identity(), Complete: result.Complete, Items: result.Items})
	}
	return render.Records(command.OutOrStdout(), api.Identity(), resource, result.Items, o.color)
}

func (o *options) showRecord(command *cobra.Command, resource, id string) error {
	if err := xero.RequireGUID(id); err != nil {
		return err
	}
	api, err := o.selectedClient(command)
	if err != nil {
		return err
	}
	record, err := api.Record(command.Context(), resource, id)
	if err != nil {
		return err
	}
	if o.json {
		return render.JSON(command.OutOrStdout(), record)
	}
	attachments := record.Text("HasAttachments")
	if attachments == "true" && (resource == "BankTransactions" || resource == "ManualJournals") {
		items, countErr := api.Attachments(command.Context(), resource, id)
		if countErr == nil {
			attachments = "yes (" + strconv.Itoa(len(items)) + ")"
		} else if command.Context().Err() != nil {
			return command.Context().Err()
		}
	}
	return render.Record(command.OutOrStdout(), resource, record, attachments, o.color)
}
