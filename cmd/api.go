package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newAPI(o *options) *cobra.Command {
	var method, input, modified string
	var params []string
	var all bool
	command := &cobra.Command{Use: "api PATH", Short: "Call an Accounting API endpoint directly", Args: cobra.ExactArgs(1),
		Long: `Call a relative Accounting API PATH, such as Accounts or Reports/BalanceSheet.
The only accepted absolute URL is https://api.xero.com/connections.
--method defaults to GET and accepts GET, POST, PUT or DELETE. Repeated
--param KEY=VALUE adds query values. --input FILE reads a JSON request body;
--input - explicitly reads stdin. No other command reads stdin.

--modified-since accepts RFC 3339 or YYYY-MM-DDTHH:mm:ss (UTC) and sends
If-Modified-Since in Xero's YYYY-MM-DDTHH:mm:ss format.
--all requires GET, pagination metadata and one top-level resource array;
it fetches pageSize=100 and combines every page into one response object.

The response goes to stdout exactly as received when redirected. Terminal
JSON is pretty-printed. --json changes neither behavior nor exit status.
On non-2xx responses, stderr contains a coded error followed by the raw
response body. No response body is written to stdout on failure.

Organisation selection, identity verification, token renewal, rate limits
and error mapping match the dedicated commands. Nothing prompts or stores
tokens. POST, PUT and DELETE can change Xero data immediately; this raw
passthrough does not add dry runs or idempotency keys.`,
		Example: `xero api Organisation
xero api Accounts --param 'where=Type=="BANK"'`,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" {
				return usage("--method must be GET, POST, PUT or DELETE")
			}
			if all && method != "GET" {
				return usage("--all requires --method GET")
			}
			request := xero.Request{Method: method, Path: args[0], Query: make(url.Values), Headers: make(http.Header)}
			for _, param := range params {
				key, value, ok := strings.Cut(param, "=")
				if !ok || key == "" {
					return usage("--param requires KEY=VALUE")
				}
				request.Query.Add(key, value)
			}
			if modified != "" {
				date, err := time.Parse(time.RFC3339, modified)
				if err != nil {
					date, err = time.Parse("2006-01-02T15:04:05", modified)
				}
				if err != nil {
					return usage("--modified-since requires RFC 3339 or YYYY-MM-DDTHH:mm:ss")
				}
				request.Headers.Set("If-Modified-Since", date.UTC().Format("2006-01-02T15:04:05"))
			}
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			if command.Flags().Changed("input") {
				if input == "-" {
					request.Body, err = io.ReadAll(command.InOrStdin())
				} else {
					request.Body, err = os.ReadFile(input)
				}
				if err != nil {
					return apperr.New("invalid_argument", "read --input %q: %v", input, err)
				}
				if request.Body == nil {
					request.Body = []byte{}
				}
			}
			var body []byte
			if all {
				_, _, body, err = xero.All(command.Context(), request, api.DoRequest)
			} else {
				_, _, body, err = api.DoRequest(command.Context(), request)
			}
			if err != nil {
				return &apiError{body: body, err: err}
			}
			if render.Terminal(command.OutOrStdout()) && json.Valid(body) {
				var formatted bytes.Buffer
				if err := json.Indent(&formatted, body, "", "  "); err != nil {
					return err
				}
				body = append(formatted.Bytes(), '\n')
			}
			_, err = command.OutOrStdout().Write(body)
			return err
		}),
	}
	command.Flags().StringVar(&method, "method", "GET", "GET, POST, PUT or DELETE")
	command.Flags().StringArrayVar(&params, "param", nil, "query KEY=VALUE; repeat to add values")
	command.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	command.Flags().StringVar(&modified, "modified-since", "", "conditional request timestamp")
	command.Flags().BoolVar(&all, "all", false, "retrieve and combine all pages")
	return command
}
