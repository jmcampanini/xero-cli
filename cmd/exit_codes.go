package cmd

import "github.com/spf13/cobra"

func newExitCodes() *cobra.Command {
	return &cobra.Command{Use: "exit-codes", Short: "Exit codes and error categories", Args: cobra.NoArgs,
		Long: `xero exits 0, 1, or 2.

  0  Success, including --help, --version, a bare group, and xero help NAME
     for an unknown NAME.
  1  Application error. Codes: not_found, invalid_argument (configuration,
     selection or values), conflict, unauthenticated (secret or token),
     forbidden (organisation mismatch or missing scope/role), rate_limited
     (with retry delay), api (other Xero failures), internal.
     Error: <message> goes to stderr, or with --json one compact value:
     {"error":{"code":"CODE","message":"TEXT"}}.
     Ordinary failures leave stdout empty. auth status prints every status
     block before returning failure; api adds the raw error body to stderr.
  2  Usage error: unknown command, flag or operand, wrong operand count,
     conflicting flags, or --json on config. Nothing is loaded and no
     request is made. Usage errors remain human-readable on stderr.

--json never changes the exit status. This topic prints help to stdout,
never loads configuration or contacts Xero, and is identical through
xero exit-codes and xero help exit-codes.`,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
}
