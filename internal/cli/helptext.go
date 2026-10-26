package cli

// helpTopicAnnotation marks an application-owned command that does no work and
// prints the same text directly or through help, so tests find topics without
// a list of names.
const helpTopicAnnotation = "help-topic"

const pagingHelp = `Paged lists fetch every page by default (100 items per request).
--page selects one page and prints page N of M (K items) on stderr.
--page-size accepts 1 to 1000 only with --page. Explicit pages are partial.`

const documentListHelp = `--where, where offered, adds a parenthesized expression with &&. Records
retain Xero's ordering. Human stdout shows the organisation and a table.
--json writes one compact object with org {name, id}, complete, and items.
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
If writing or closing fails, the command removes the new output file.
Use --output PATH for another destination or --output - for exact bytes on
stdout, without a trailing newline. --json is not supported for downloads.
File downloads leave stdout empty. Errors go to stderr. Requires attachment
read access. Nothing prompts or reads stdin.`

// Shared fragments compose command long descriptions so repeated contracts cannot drift.
const readHelp = `This command never writes accounting records or stores tokens. It verifies
the configured organisation before calling the Accounting API. Errors go to
stderr; successful payloads go to stdout. Nothing prompts.

` + credentialFileHelp

const credentialFileHelp = `Authentication reads the configured secret_file of each accessed org; see
xero config --help for its path resolution and permission requirements.`

const groupHelp = `With no subcommand, print help to stdout and exit successfully. Unexpected
operands are usage errors on stderr. Help reads no configuration, makes no
requests and never prompts. --json does not change help output.`

const listHelp = `--json writes one compact object with org {name, id}, complete: true and
items containing native Xero objects with dates normalized. Human output
shows the organisation and a table. These resources are not paginated.`
