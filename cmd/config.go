package cmd

import (
	"github.com/jmcampanini/xero-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfig(o *options) *cobra.Command {
	var provenance bool
	command := &cobra.Command{Use: "config", Short: "Print effective TOML configuration", Args: cobra.NoArgs,
		Long: `Print effective configuration as TOML, suitable for redirection into a file.
--provenance appends source comments to values. --json is a usage error;
TOML is this command's machine format. Errors go to stderr.

Precedence is defaults, config file, XERO_ORG, then --org. The discovered
file is $XDG_CONFIG_HOME/xero/config.toml, or ~/.config/xero/config.toml.
A missing discovered file is allowed. --config replaces discovery and must
exist. A relative XDG_CONFIG_HOME currently resolves against the working
directory. No config files, secrets or tokens are created by this command.

default_org = "acme"

[orgs.acme]
client_id = "00000000000000000000000000000000"
secret_file = "~/.config/xero/secrets/acme"
organisation_id = "00000000-0000-0000-0000-000000000000"
scopes = []

Each org needs a 32-hex client_id and secret_file. Names contain lowercase
letters or digits separated by single hyphens. Relative secret paths resolve
against the config file's directory; ~/ expands to the home directory.
Secret files must be regular files with no group or other permission bits.
The entire content is trimmed and must be nonempty. Keep secrets outside Git.
This report prints the resolved path, never the secret, and does not read it.

A Custom Connection's scopes are fixed in the developer portal. The optional
scopes key exists only for connections that require an explicit request.
An empty list omits scope from the token request.

API commands use --org, XERO_ORG or default_org, or the sole configured org.
With multiple orgs and no selection they fail before any request. Run
auth status NAME to discover the organisation_id before setting it here.
This report preserves default_org as configured; it does not persist a
derived selection, so redirecting it cannot silently choose an organisation.`,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			if o.json {
				return usage("--json is not supported by config; TOML is its machine format")
			}
			cfg, report, err := config.Load(command.Flags(), o.configPath)
			if err != nil {
				return err
			}
			body, err := config.TOML(cfg, report, provenance)
			if err != nil {
				return err
			}
			_, err = command.OutOrStdout().Write(body)
			return err
		}),
	}
	command.Flags().BoolVar(&provenance, "provenance", false, "annotate values with their source")
	return command
}
