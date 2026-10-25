package cli

const documentListHelp = `Paged lists fetch every page by default (100 items per request).
--page selects one page and prints page N of M (K items) on stderr.
--page-size accepts 1 to 1000 only with --page. --where, where offered,
adds a parenthesized expression with &&. Records retain Xero's ordering.
Human stdout shows the organisation and a table. --json writes one compact
object with org {name, id}, complete, and items. Explicit pages are partial.
Source fields and native tracking names are preserved; money is a decimal
string. Accounting dates stay YYYY-MM-DD and timestamps are RFC 3339 UTC.
Nothing prompts, reads stdin or writes files.`

const documentShowHelp = `ID must be a GUID, validated before configuration or network access.
Human stdout shows saved fields and line details, with tracking names as
Category=Option. Attachment presence is always shown; counts are best effort
when available. Missing attachment access does not prevent showing a record.
--json writes the native Xero object with every source field, native tracking
names, decimal money strings and UTC dates. It adds no tracking aliases.
Accounting dates stay YYYY-MM-DD; timestamps are RFC 3339 UTC.
Nothing prompts, reads stdin or writes files.`

const attachmentsHelp = `List attachment IDs, names, MIME types and byte lengths for this GUID.
--json writes org {name, id}, complete: true and native attachment items.
Payloads go to stdout; errors go to stderr. Requires attachment read access.
Nothing prompts, reads stdin or writes files.`

const attachmentHelp = `Download the exact attachment FILENAME on the document identified by GUID.
The file's MIME type comes from its attachment listing. --output defaults
to FILENAME in the working directory; an existing path is never overwritten.
Use --output PATH for another destination or --output - for exact bytes on
stdout, without a trailing newline. --json is not supported for downloads.
File downloads leave stdout empty. Errors go to stderr. Requires attachment
read access. Nothing prompts or reads stdin.`

// Shared fragments compose command long descriptions so repeated contracts cannot drift.
const readHelp = `This command never writes accounting records or stores tokens. It verifies
the configured organisation before calling the Accounting API. Errors go to
stderr; successful payloads go to stdout. Nothing prompts.`

const groupHelp = `With no subcommand, print help and exit successfully. Unexpected operands
are usage errors. Help reads no configuration and makes no requests.
--json does not change help output.`

const listHelp = `--json writes one compact object with org {name, id}, complete: true and
items containing native Xero objects with dates normalized. Human output
shows the organisation and a table. These resources are not paginated.`
