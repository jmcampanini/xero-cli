package cmd

import "github.com/spf13/cobra"

func newCompletion() *cobra.Command {
	return &cobra.Command{Use: "completion SHELL", Short: "Generate shell completion", ValidArgs: []string{"bash", "zsh", "fish", "powershell"}, Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Long: `Write a completion script for bash, zsh, fish or powershell to stdout.
Redirect it to the appropriate shell completion directory. Errors go to
stderr. --json does not change the script format. This command never reads
configuration, creates files or contacts Xero.`,
		RunE: func(command *cobra.Command, args []string) error {
			var err error
			switch args[0] {
			case "bash":
				err = command.Root().GenBashCompletionV2(command.OutOrStdout(), true)
			case "zsh":
				err = command.Root().GenZshCompletion(command.OutOrStdout())
			case "fish":
				err = command.Root().GenFishCompletion(command.OutOrStdout(), true)
			case "powershell":
				err = command.Root().GenPowerShellCompletionWithDesc(command.OutOrStdout())
			}
			if err != nil {
				return &applicationError{err}
			}
			return nil
		},
	}
}
