package cmd

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
