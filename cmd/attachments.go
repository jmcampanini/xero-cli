package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/spf13/cobra"
)

func (o *options) showAttachments(command *cobra.Command, resource, id string) error {
	if err := requireGUID(id); err != nil {
		return err
	}
	api, err := o.selectedClient(command)
	if err != nil {
		return err
	}
	items, err := api.Attachments(command.Context(), resource, id)
	if err != nil {
		return err
	}
	var rows [][]string
	for _, item := range items {
		rows = append(rows, []string{item.Text("FileName"), item.Text("MimeType"), item.Text("ContentLength"), item.Text("AttachmentID")})
	}
	return o.list(command, api.Identity(), items, []string{"FILENAME", "MIME TYPE", "BYTES", "ID"}, rows)
}

func (o *options) downloadAttachment(command *cobra.Command, resource, id, name, output string) error {
	if err := requireGUID(id); err != nil {
		return err
	}
	if o.json {
		return usage("--json is not supported for attachment downloads")
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return apperr.New("invalid_argument", "FILENAME must be a file name without path separators")
	}
	if command.Flags().Changed("output") && output == "" {
		return usage("--output must not be empty")
	}
	if !command.Flags().Changed("output") {
		output = "./" + name
	}
	if output != "-" {
		if _, err := os.Lstat(output); err == nil {
			return apperr.New("invalid_argument", "refusing to overwrite %q", output)
		} else if !errors.Is(err, os.ErrNotExist) {
			return apperr.New("invalid_argument", "inspect output %q: %v", output, err)
		}
	}
	api, err := o.selectedClient(command)
	if err != nil {
		return err
	}
	body, err := api.Attachment(command.Context(), resource, id, name)
	if err != nil {
		return err
	}
	if output == "-" {
		_, err := command.OutOrStdout().Write(body)
		return err
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return apperr.New("invalid_argument", "create output %q without overwriting: %v", output, err)
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		removeErr := os.Remove(output)
		return apperr.New("internal", "write output %q: %v", output, errors.Join(err, removeErr))
	}
	return nil
}
