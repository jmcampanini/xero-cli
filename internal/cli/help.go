package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newHelp(o *options) *cobra.Command {
	return &cobra.Command{Use: "help [command]", Short: "Show help for a command or topic", Args: cobra.ArbitraryArgs,
		Long: `Write help for a command path or topic to stdout. With no path, show root
help. An unknown topic prints a notice and root usage, then exits 0.
Errors go to stderr. --json does not change the text format. Help never
loads configuration, reads stdin, writes files, prompts or contacts Xero.`,
		ValidArgsFunction: func(command *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			target, _, err := command.Root().Find(args)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var completions []cobra.Completion
			for _, child := range target.Commands() {
				if (child.IsAvailableCommand() || child == command) && strings.HasPrefix(child.Name(), toComplete) {
					completions = append(completions, cobra.CompletionWithDesc(child.Name(), child.Short))
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: o.run(func(command *cobra.Command, args []string) error {
			target, _, err := command.Root().Find(args)
			if target == nil || err != nil {
				if _, err := fmt.Fprintf(command.OutOrStdout(), "Unknown help topic %#q\n", args); err != nil {
					return err
				}
				return command.Root().Usage()
			}
			target.SetContext(command.Context())
			target.InitDefaultHelpFlag()
			return target.Help()
		}),
	}
}
