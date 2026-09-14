// Package cmd constructs a fresh Cobra tree and adapts errors to process exit codes.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/config"
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

// Version is injected at build time with the commit-derived release identity.
var Version = "unknown"

type client interface {
	Identity() xero.Identity
	AuthStatus(context.Context) (xero.AuthStatus, error)
	Organisation(context.Context) (xero.Organisation, error)
	Report(context.Context, string, url.Values) (xero.Report, error)
	Accounts(context.Context, url.Values) ([]xero.Account, error)
	Account(context.Context, string) (xero.Account, error)
	TrackingCategories(context.Context) ([]xero.TrackingCategory, error)
	DoRequest(context.Context, xero.Request) (int, http.Header, []byte, error)
}

type options struct {
	color      string
	configPath string
	factory    func(string, config.OrgConfig) client
	json       bool
}

type applicationError struct{ error }
type usageError struct{ error }

func usage(format string, args ...any) error { return &usageError{fmt.Errorf(format, args...)} }

// Execute runs with process streams; only main owns os.Exit.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return execute(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, nil)
}

func execute(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, factory func(string, config.OrgConfig) client) int {
	settings := &options{factory: factory}
	root, err := newRoot(settings)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "Error: %s\n", err)
		return 1
	}
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	err = root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	var usageErr *usageError
	var appErr *applicationError
	if errors.As(err, &usageErr) || !errors.As(err, &appErr) {
		_, _ = fmt.Fprintf(errOut, "Error: %s\n", strings.Join(strings.Fields(err.Error()), " "))
		return 2
	}
	coded := apperr.From(appErr.error)
	coded = &apperr.Error{Code: coded.Code, Message: strings.Join(strings.Fields(coded.Message), " ")}
	if settings.json {
		_ = render.JSON(errOut, map[string]any{"error": coded})
	} else {
		_, _ = fmt.Fprintf(errOut, "Error: %s\n", coded.Message)
	}
	var rawErr *apiError
	if errors.As(appErr.error, &rawErr) && len(rawErr.body) > 0 {
		_, _ = errOut.Write(rawErr.body)
		if rawErr.body[len(rawErr.body)-1] != '\n' {
			_, _ = io.WriteString(errOut, "\n")
		}
	}
	return 1
}

func newRoot(o *options) (*cobra.Command, error) {
	if o.factory == nil {
		budget := xero.NewBudget()
		o.factory = func(name string, org config.OrgConfig) client {
			return xero.New(xero.Options{Budget: budget, ClientID: org.ClientID, Name: name, OrganisationID: org.OrganisationID, Scopes: org.Scopes, SecretFile: org.SecretFile, Version: Version})
		}
	}
	root := &cobra.Command{Use: "xero", Short: "Inspect Xero settings, accounts, tracking and reports", Version: Version, SilenceUsage: true, SilenceErrors: true, DisableSuggestions: true,
		Long: `Read Xero settings, chart of accounts, tracking categories and reports.
Use api to call Accounting API endpoints without a dedicated command.

Configure a Custom Connection per organisation with xero config --help.
Tokens stay in memory; every run verifies the selected organisation.
Nothing prompts. Dedicated resource commands only read data; api can write.
The installed CLI requires no external programs.

Payloads go to stdout and diagnostics to stderr. --json writes compact JSON
for resource commands; config writes TOML and api retains the raw response.
Use xero help exit-codes for error categories and process exit statuses.`,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	root.PersistentFlags().BoolVar(&o.json, "json", false, "write machine-readable JSON")
	root.PersistentFlags().StringVar(&o.color, "color", "auto", "human output color: auto, always, never")
	root.PersistentFlags().StringVar(&o.configPath, "config", "", "required TOML configuration file")
	if err := pflagloader.Register[config.Config](root.PersistentFlags()); err != nil {
		return nil, err
	}
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newConfig(o), newExitCodes(), newCompletion(), newAuth(o), newOrgs(o), newOrg(o), newAPI(o), newAccounts(o), newAccount(o), newTrackingCategories(o), newTrackingCategory(o), newReport(o))
	return root, nil
}

func (o *options) run(fn func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(command *cobra.Command, args []string) error {
		if o.color != "auto" && o.color != "always" && o.color != "never" {
			return usage("--color must be auto, always or never")
		}
		err := fn(command, args)
		if err == nil {
			return nil
		}
		var badUsage *usageError
		if errors.As(err, &badUsage) {
			return err
		}
		return &applicationError{err}
	}
}

func (o *options) load(command *cobra.Command) (config.Config, error) {
	cfg, _, err := config.Load(command.Flags(), o.configPath)
	return cfg, err
}

func (o *options) selectedClient(command *cobra.Command) (client, error) {
	cfg, err := o.load(command)
	if err != nil {
		return nil, err
	}
	name, err := cfg.Effective()
	if err != nil {
		return nil, err
	}
	if err := config.RequireOrganisationID(name, cfg.Orgs[name].OrganisationID); err != nil {
		return nil, err
	}
	return o.factory(name, cfg.Orgs[name]), nil
}
