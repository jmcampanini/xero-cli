package cmd

import (
	"fmt"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newAuthStatus(o *options) *cobra.Command {
	return &cobra.Command{Use: "status [NAME]", Short: "Diagnose credentials, identity and command access", Args: cobra.MaximumNArgs(1),
		Long: `Inspect NAME, or every configured organisation when NAME is omitted.
Print credential-file, token, organisation, scope, command and rate-limit
diagnostics. The token's scope and exp claims are decoded for display only;
Xero validates the token. Tokens and secrets are never printed or stored.

An unset organisation_id is allowed here so you can copy the discovered ID
into config. A mismatch is a failure. Missing accounts or tracking scopes
fail this check; missing scopes for later commands are informational.
Granular report access means at least one report, not every report.

Human status blocks go to stdout. --json writes an array with name,
client_id, secret_file, secret, secret_ok, token, token_ok, organisation_id,
organisation_name, organisation, organisation_ok, scopes, commands and
limits as strings or booleans. Unavailable checks are marked explicitly.
Rate limits are unavailable when Xero omits the response headers; this
command does not make extra accounting requests to obtain them.

Every organisation is reported before the first failing org's error is
written to stderr and the process exits 1. --org does not restrict this
diagnostic; use the NAME operand. This command never changes Xero data.`,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			cfg, err := o.load(command)
			if err != nil {
				return err
			}
			names := cfg.Names()
			if len(args) > 0 {
				if _, ok := cfg.Orgs[args[0]]; !ok {
					return apperr.New("invalid_argument", "organisation %q is not configured", args[0])
				}
				names = args
			}
			statuses := make([]xero.AuthStatus, 0, len(names))
			var firstErr error
			for _, name := range names {
				status, err := o.factory(name, cfg.Orgs[name]).AuthStatus(command.Context())
				statuses = append(statuses, status)
				if firstErr == nil {
					firstErr = err
				}
			}
			if o.json {
				if err := render.JSON(command.OutOrStdout(), statuses); err != nil {
					return err
				}
			} else {
				for i, status := range statuses {
					if i > 0 {
						if _, err := fmt.Fprintln(command.OutOrStdout()); err != nil {
							return err
						}
					}
					if _, err := fmt.Fprintf(command.OutOrStdout(), "%s  %s  (%s)\n", status.Name, status.OrganisationName, status.OrganisationID); err != nil {
						return err
					}
					rows := [][]string{{"  client id", status.ClientID}, {"  secret file", status.SecretFile + "  (" + status.Secret + ")"}, {"  token", status.Token}, {"  organisation", status.Organisation}, {"  scopes", status.Scopes}, {"  commands", status.Commands}, {"  limits", status.Limits}}
					if err := render.Table(command.OutOrStdout(), nil, rows, o.color); err != nil {
						return err
					}
				}
			}
			return firstErr
		}),
	}
}
