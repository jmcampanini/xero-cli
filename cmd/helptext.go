package cmd

const reportHelp = `Human stdout has organisation, period, currency and applicable basis or
tracking context, then sectioned rows with account codes and decimal values.
--json writes one compact object with context, columns and flattened rows;
values are decimal strings. Bank Summary omits filters and by as well.
Native API layouts can differ from Xero's current web layouts, including
account placement, signs and section subtotals. Values are not rewritten.
--csv writes section,label,account_code and value columns to stdout;
context goes to stderr. CSV and JSON are mutually exclusive. Only human
output uses color or thousands separators. Errors go to stderr.

Reports need settings access to resolve account codes and base currency,
and report access with the Xero user's Reports role. Requests verify the
configured organisation and share the process's rate limit. Nothing prompts,
reads stdin or writes files. Redirect stdout to save JSON or CSV.`

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
