// Command xero is the Xero Accounting CLI entry point. It owns the process
// exit and leaves all command behavior to internal/cli.
package main

import (
	"os"

	"github.com/jmcampanini/xero-cli/internal/cli"
)

func main() { os.Exit(cli.Execute()) }
