# Command surface

`xero` is a Go CLI over the Xero Accounting API for keeping the books of
one or more small businesses, each in its own Xero organisation. The scope
is the monthly entry loop (Receive Money, Spend Money, transfers,
contacts, read-back) and the reporting and year-end loop (reports by
tracking option, account transactions, manual journals, chart of
accounts). `DOMAIN.md` holds the API and prior-art evidence this rests on.

Examples use two placeholder organisations, `acme` and `beta`, a tracking
category named `Region` with options `North` and `South`, and account
codes from Xero's default chart (`090` Business Bank Account, `200`
Sales, `260` Other Revenue, `400` Advertising, `429` General Expenses,
`469` Rent, `620` Prepayments).


## Grammar

- **Noun-verb, one spelling per action.** No aliases, no sugar verbs.
- **Plural noun operates on the collection**: `xero accounts list`,
  `xero manual-journals add`. A bare plural prints help.
- **Singular noun operates on one entity** and its first operand is that
  entity's identity: `xero account show 200`,
  `xero manual-journal post ID`.
- **State changes are verbs, not status flags.** `post`, `void`,
  `archive`, `delete` each exist as a verb. `edit` never changes status.
- **Same field, same flag, everywhere.** `--date`, `--from`, `--to`,
  `--month`, `--contact`, `--account`, `--reference`, `--line`,
  `--basis` mean the same thing on every command that has them.
- **Reports are a group with one verb per Xero report**:
  `xero report balance-sheet`. Singular because each run produces one
  report.
- **Kebab-case nouns** matching Xero's names: `bank-transactions`,
  `manual-journals`, `tracking-categories`.
- **Nothing prompts.** Writes name the organisation and the affected
  record on stderr, accept `--dry-run`, and carry an idempotency key.
- **Every result states its context.** Reports and lists carry the
  organisation, date range, basis, currency, and filters in human and
  JSON output, so a saved result can be matched to Xero later.


## Organisations

An organisation is a company's Xero account, and a named connection in
config binds one Xero app (client ID, secret file) to one verified
organisation ID.

| Name | Xero app | Organisation | Grant |
| --- | --- | --- | --- |
| `acme` | a Custom Connection | Acme Holdings Ltd | client credentials |
| `beta` | a Custom Connection | Beta Ventures Ltd | client credentials |

Selection precedence: `--org NAME`, then `XERO_ORG`, then `default_org`
in config. When more than one organisation is configured and none is
selected, every command that calls the API fails with `invalid_argument`
before any request. There is no fallback to the first connection.

Every run verifies identity: after obtaining a token the client calls
`/connections`, requires exactly the configured organisation ID to be
present, and sends it on every request (Xero's `xero-tenant-id` header).
A mismatch is `forbidden` and stops before any accounting call. Writes
print `org: Acme Holdings Ltd (ID)` on stderr with the record ID.


## Addressing

| Entity | Identity on the command line | Why |
| --- | --- | --- |
| Organisation | configured name (`acme`, `beta`) | few of them; names are memorable |
| Account | account code, or the account name when the code is empty, or the GUID | codes are what the chart and journals show |
| Tracking category | name, case-insensitive | at most two active per org; never assumed to match across orgs |
| Tracking option | name within its category, case-insensitive | up to 100, unique within the category |
| Contact | ContactID GUID | names are not unique; `contacts list --search` finds the ID |
| Bank transaction, transfer, manual journal | GUID | no other identity exists |

A value shaped like a GUID is always treated as an ID. Name lookups are
exact after case folding; an ambiguous name is `invalid_argument` naming
the candidates, never a guess.


## Global flags and configuration

```
--org NAME          organisation for this invocation
--json              machine-readable output on stdout, one value, one line
--color MODE        auto, always, never; human stdout only
--config PATH       TOML file; must exist when given
```

Precedence is defaults, then `$XDG_CONFIG_HOME/xero/config.toml`, then
`XERO_*` environment variables, then flags.

```toml
default_org = "acme"

[orgs.acme]
client_id = "00000000000000000000000000000000"
secret_file = "~/.config/xero/secrets/acme"      # client secret, one line
organisation_id = "00000000-0000-0000-0000-000000000000"

[orgs.beta]
client_id = "11111111111111111111111111111111"
secret_file = "/somewhere/outside/git/beta-secret"
organisation_id = ""                             # `auth status` prints it
```

The TOML holds identifiers. The client secret lives in the file
`secret_file` names, whole content trimmed, and that path can be anywhere
outside version control. A secret file readable by group or others is
refused with `unauthenticated` naming the file. No token is stored: every
run fetches a 30-minute access token from the secret, so a restart or an
expiry changes nothing. `xero config`, `auth status`, diagnostics, and
logs redact the secret. A `~` prefix and a relative path resolve against
the home directory and the config file's directory respectively.

```
xero config [--provenance]        effective configuration as redirectable TOML
xero exit-codes                    help topic
xero completion SHELL
```


## Core: what the tool exists for

Reads before writes in every slice.


### Auth and organisations

```
xero auth status [NAME]
xero orgs
xero org show
```

- There is no sign-in command for a Custom Connection: the config names
  the app and the secret file, and every run authenticates from them.
  Client-credentials tokens last 30 minutes and have no refresh token;
  the client fetches one per run and retries once on a 401.
- `auth status` prints, per organisation: client ID, secret file and
  whether it is readable, the organisation ID and name Xero answered
  with, whether that matches config, the scopes the token carries with
  each command group marked available or missing, and the remaining
  daily and per-minute call budget from the last response. A missing
  secret file, a wrong secret, a revoked connection, and insufficient
  scope each produce a distinct, named error; none is ever an empty
  list. With `organisation_id` unset it prints the discovered ID so it
  can be pasted into config; API commands refuse to run until it is set.
- `orgs` prints configured organisations with the effective one marked.
  `org show` prints the Organisation record (name, country, base
  currency, financial year end, tax basis, organisation ID) for the
  effective organisation.
- Browser authorisation (`auth login`) is a later slice and never a
  prerequisite.


### API passthrough

```
xero api PATH [--method GET|POST|PUT|DELETE] [--param KEY=VALUE]...
              [--input FILE|-] [--modified-since DATETIME] [--all]
```

Every endpoint with the same organisation selection, identity check,
token renewal, rate limiting, error mapping, and exit codes as the rest of
the tool, with the raw Xero JSON on stdout. `--all` follows `page` until
the last page and concatenates the resource array. It makes every resource
reachable on day one and stays the escape hatch. It is also how the rare
reference lookups (tax rates, currencies, items) are done.


### Chart of accounts

```
xero accounts list [--type TYPE] [--class CLASS] [--bank]
                   [--status active|archived|all]
xero accounts add NAME --code CODE --type TYPE [--tax TAXTYPE]
                       [--description TEXT] [--enable-payments] [--dry-run]
xero account show CODE [--date DATE] [--basis cash|accrual]
xero account edit CODE [--name TEXT] [--code CODE] [--description TEXT]
                       [--tax TAXTYPE] [--enable-payments|--no-enable-payments]
                       [--dry-run]
xero account archive CODE
xero account transactions CODE (--from DATE --to DATE | --month YYYY-MM)
                               [--tracking CAT=OPT]... [--include-deleted] [--csv]
```

- `account show --date` adds the account's balance at that date from the
  trial balance. Without it, no report call is made, and an explicit
  `--basis` is a usage error. Balances use exact YTD debit/credit
  subtraction, with financial-year-to-date labels for income and expenses.
- `account transactions` is cash-only and supports the organisation's base
  currency. It combines bank transactions and deduplicated bank transfers
  for bank accounts, or matching bank-transaction and cash-enabled posted
  manual-journal lines for other accounts. Invoice/bill payments and refunds,
  prepayment/overpayment allocations and refunds, and system-generated lines
  are excluded and always disclosed; JSON has `complete:false`.
- Unfiltered output has a cash trial-balance opening, class-signed running
  balances, and a reconstructed closing balance. Income/expense balances are
  financial-year-to-date, zero at the fiscal-year start, and require a range
  within one financial year. For ranges ending today or earlier, compare
  against the closing cash trial balance and show the unexplained difference.
- `--tracking` shows matching rows and net movement only, with no balance
  fields or trial-balance requests. Bank rows retain whole-document amounts.
  This mode and balance-sheet accounts permit ranges crossing financial years.
- Rows include date, source, contact or narration, description, reference,
  debit, credit, tracking and status, plus balance when unfiltered. CSV splits
  source type, subtype and full ID into separate columns. Deleted/voided rows
  appear only with `--include-deleted`, marked and excluded from calculations.


### Tracking categories and options

```
xero tracking-categories list [--archived|--all]
xero tracking-categories add NAME
xero tracking-category show NAME
xero tracking-category rename OLD NEW
xero tracking-category archive NAME
xero tracking-category delete NAME
xero tracking-options add CATEGORY NAME...
xero tracking-option rename CATEGORY OLD NEW
xero tracking-option archive CATEGORY NAME
xero tracking-option delete CATEGORY NAME
```

The category is name-addressed, the option is addressed by category then
name. `show` lists options with status, so archived options stay visible
for historical reports. Each organisation's structure is discovered,
never assumed. Xero allows two active categories; a third `add` is a
`conflict`.


### Reports

```
xero report profit-and-loss  (--from DATE --to DATE | --month YYYY-MM | --year YYYY)
                             [--periods N] [--timeframe month|quarter|year]
                             [--basis cash|accrual]
                             [--tracking CAT=OPT]... [--by CATEGORY]
                             [--standard-layout]
xero report balance-sheet    [--date DATE] [--periods N] [--timeframe ...]
                             [--basis cash|accrual]
                             [--tracking CAT=OPT]... [--by CATEGORY]
                             [--standard-layout]
xero report trial-balance    [--date DATE] [--basis cash|accrual]
xero report bank-summary     (--from DATE --to DATE | --month YYYY-MM)
```

Every `report` command and `account transactions` also take `--csv`.

- `--basis` maps to `paymentsOnly` and defaults to `accrual`, Xero's
  default. The chosen basis is printed in the report header and JSON so
  a cash profit and loss is never mistaken for accrual. Changing layout
  or columns never changes the basis.
- `--month YYYY-MM` and `--year YYYY` expand to exact calendar ranges;
  they are mutually exclusive with `--from`/`--to`. A range that is not
  whole months combined with `--timeframe month` is `invalid_argument`
  rather than a silently partial comparison.
- `--periods` with `--timeframe` adds comparison columns; every column
  Xero returns is kept in every output mode. The supplied date range
  applies to each native comparison; a year is not split into months.
  Whole-period validation does not correct Xero's truncation of earlier
  31-day months when the base month has 30 days. For twelve monthly
  columns ending in December, use `--month YYYY-12 --periods 11
  --timeframe month` and verify the returned captions.
- `--tracking CAT=OPT` filters to one option (`trackingOptionID1/2` on
  the balance sheet, `trackingCategoryID` plus `trackingOptionID` on
  profit and loss). Names resolve to IDs through one `TrackingCategories`
  call per run. An unknown or ambiguous name is `invalid_argument`; the
  command never returns an unfiltered report in its place.
- `--by CATEGORY` produces one column per option plus `Unassigned` and
  `Total`. Profit and loss does this natively. Balance sheet cannot, so
  the tool issues one call per active option plus the unfiltered total
  and selects the requested date from each response before laying the
  columns side by side. Native balance sheets include a previous-year
  column even without comparison parameters. Help states that untagged
  balances appear only in the total column and the columns do not foot. `--by`
  and `--tracking` on the same category is a usage error. `--by` always
  takes the category name; nothing is implied from config.
- `bank-summary` is the monthly cash-movement view: opening balance,
  received, spent, closing balance per bank account for the exact range.
  Help states it is not a profit and loss and that transfers count as
  movements. It is the entry point for `account transactions` when a
  movement needs explaining. Its endpoint has no accounting-basis option,
  so its output omits basis, filters and breakdown metadata.
- Human output keeps the section structure as a table. `--json` is a
  flattened shape owned by the tool: organisation, report name, basis,
  dates, filters, column captions, then rows with section path, label,
  account code and ID when present, and one value per column as a
  decimal string. `--csv` writes the same rows as CSV; `--csv` and
  `--json` together is a usage error.


### Manual journals

```
xero manual-journals list [--status draft|posted|voided|all]
                          (--from DATE --to DATE | --month YYYY-MM | --year YYYY)
                          [--modified-since DATETIME] [--where EXPR]
xero manual-journals add --narration TEXT --date DATE
                         --line SPEC... [--amounts exclusive|inclusive|no-tax]
                         (--cash-basis | --no-cash-basis)
                         [--post] [--file FILE|-]
                         [--idempotency-key KEY] [--dry-run]
xero manual-journal show ID
xero manual-journal edit ID [--narration TEXT] [--date DATE] [--line SPEC...]
                            [--amounts ...] [--cash-basis|--no-cash-basis]
                            [--file FILE|-] [--idempotency-key KEY] [--dry-run]
xero manual-journal post ID
xero manual-journal void ID
xero manual-journal delete ID
```

- `--line SPEC` repeats once per journal line. A spec is space-separated
  `key=value` pairs, shell-quoted as one operand:
  `--line 'account=469 debit=1200 description="March rent" tracking=Region=North'`.
  Keys: `account` (code), one of `debit` or `credit`, `description`,
  `tax` (tax type, default the account's), `tracking` (repeatable within
  a spec, always `CATEGORY=OPTION`; the category is never implied).
  Debits and credits must balance; an unbalanced journal is
  `invalid_argument` before any request. `--file` takes the Xero JSON
  body for anything the spec cannot say; mixing `--file` with `--line`
  is a usage error.
- `--cash-basis` maps to `ShowOnCashBasisReports`. It has no default:
  `add` requires one of the pair so cash treatment is deliberate.
- `add` creates a draft unless `--post`. `edit` and `delete` need a
  draft; `void` needs a posted journal. The wrong state is a `conflict`.
  `edit` replaces only the fields given; lines are replaced as a set when
  `--line` or `--file` is present and otherwise untouched.
- Every write sends an `Idempotency-Key`. The tool generates one when
  `--idempotency-key` is absent and prints it on stderr with the result,
  so a retry after an uncertain response can reuse it and Xero returns
  the original record instead of a duplicate.
- `show` prints every line with account, amounts, tax type, and tracking
  names, and the accounting date exactly as stored (`YYYY-MM-DD`, never
  converted to local time). It is the verification step after a write.


## Basics: monthly entry work

A typical month is one Receive Money per customer and one Spend Money
per supplier, each with several lines coded to different accounts and
tracking options, followed by reading each saved record back, and
matching transfers between bank accounts before creating new ones.


### Bank transactions and transfers

```
xero bank-transactions list --account CODE
                            (--from DATE --to DATE | --month YYYY-MM)
                            [--type spend|receive|transfer|all]
                            [--contact ID] [--reference TEXT] [--amount N]
                            [--unreconciled] [--include-deleted] [--where EXPR]
xero bank-transactions receive --account CODE --contact ID --date DATE
                               --line SPEC... [--reference TEXT]
                               [--amounts exclusive|inclusive|no-tax]
                               [--file FILE|-] [--idempotency-key KEY] [--dry-run]
xero bank-transactions spend   ...same flags as receive...
xero bank-transaction show ID
xero bank-transaction edit ID [--contact ID] [--date DATE] [--reference TEXT]
                              [--line SPEC...] [--amounts ...] [--file FILE|-]
                              [--idempotency-key KEY] [--dry-run]
xero bank-transaction delete ID
xero bank-transaction attachments ID
xero bank-transaction attachment ID FILENAME [--output PATH|-]
xero bank-transfers list (--from DATE --to DATE | --month YYYY-MM)
xero bank-transfers add --from-account CODE --to-account CODE --amount N
                        --date DATE [--reference TEXT]
                        [--idempotency-key KEY] [--dry-run]
xero bank-transfer show ID
```

- `receive` and `spend` are the two creation verbs, matching the Xero
  UI's Receive Money and Spend Money. There is no `add --type`.
- Line specs use the same grammar as manual journals with `amount`
  instead of `debit`/`credit`, plus `quantity` (default 1) and `item`:
  `--line 'account=200 amount=1440 description="March sale" tracking=Region=North'`.
  Tax type defaults to the account's; `tax=NONE` is explicit.
- Bank transactions and manual journals retain Xero's date order. Order
  within a date is unspecified; explicit pages remain Xero's pages. Bank
  transfers order by date then ID.
- `--reference` matches literal text, including quotes and backslashes.
  Xero filter strings double embedded quotes and preserve backslashes.
- `list --contact --reference --amount` exist for the duplicate check
  before entry. Deleted records are hidden unless `--include-deleted`
  and then marked, so a replaced entry is never counted twice.
- `--unreconciled` filters on `IsReconciled`. Help states that bank
  statement lines and reconciliation itself are not in the API.
- `show` prints lines with account, quantity, unit amount, tax type, tax
  amount, tracking names, the stored accounting date, reference, status,
  and reconciled flag, which is the post-save verification step.


### Contacts

```
xero contacts list [--search TEXT] [--customers] [--suppliers]
                   [--status active|archived|all] [--where EXPR]
xero contacts add NAME [--email TEXT] [--phone TEXT] [--file FILE|-]
                       [--idempotency-key KEY] [--dry-run]
xero contact show ID
xero contact edit ID [--name TEXT] [--email TEXT] [--phone TEXT] [--file FILE|-]
                     [--idempotency-key KEY] [--dry-run]
xero contact archive ID
```

`show` includes the contact's default sales and purchase tax types and
tracking, since selecting a contact in Xero applies those defaults to a
new transaction.


## Common list behaviour

- Every list fetches every page by default and the JSON carries
  `"complete": true`. `--page N [--page-size N]` returns one page,
  prints `page N of M` on stderr, and sets `"complete": false`. A list
  is never silently partial.
- `--where EXPR` and `--order FIELD` pass Xero's filter grammar through
  on endpoints that support it; `--modified-since` sets
  `If-Modified-Since`.
- Date filters use accounting dates. `--month` expands to the whole
  calendar month.


## Later: available, not planned for v1

- Browser authorisation (`auth login`) for a user without a Custom
  Connection.
- `history` on any document.
- `--csv` on `list` commands.
- A response cache with `--no-cache`, once the daily limit bites.
- `export` of every resource to dated JSONL.
- A `describe` command emitting the command schema for agents.
- Relative dates (`today`, `last-month`) and financial-year forms
  (`--fy 2025`) resolved against the organisation's year end.
- The general ledger feed, last: `journals list [--from NUMBER]
  [--modified-since DATETIME] [--basis cash|accrual] [--account CODE]`
  and `journal show NUMBER|ID`, available only when the organisation's
  app carries `accounting.journals.read`, and `account transactions`
  switching to the feed when it does. Nothing earlier depends on it.


## Excluded

- Bank reconciliation and bank statement lines: not in the Accounting
  API. `--unreconciled` on Xero-side records is the limit.
- Invoices, bills, payments, quotes, credit notes, purchase orders,
  repeating invoices, contact groups, overpayments, prepayments, batch
  payments: invoicing is a different workflow from bank-transaction
  bookkeeping and is out of scope.
- Aged receivables and payables, budgets, budget summary, executive
  summary: invoice-based or not part of either loop.
- Items, tax rates, currencies, users, branding themes: read-only
  reference data reachable through `xero api`.
- Deprecated endpoints: receipts, expense claims, payment services.
- Payroll, Projects, Files, Assets, Bank Feeds, Finance API.
- Regional tax reports (GST/BAS, 1099). Linked transactions.
- Named profiles bound to organisations, a local sync store, YAML, TSV,
  TOON.


## Output contract

- Human output on stdout, diagnostics on stderr as `Error: <message>`.
- `--json` writes one compact value on one line. Entity commands emit the
  Xero object for that resource with `/Date(ms)/` values normalised to
  ISO 8601 dates (accounting dates stay `YYYY-MM-DD`; UTC timestamps stay
  UTC), and the `pagination` block replaced by `complete`. Tracking keeps
  Xero's original field names and every returned field, including unknown
  fields. Do not rename tracking fields or add normalized aliases. Line
  tracking retains `Name`, `Option`, `TrackingCategoryID`, and
  `TrackingOptionID` when returned; contact defaults retain
  `TrackingCategoryName` and `TrackingOptionName`. Human output renders
  line tracking as `Category=Option`. Lists are
  `{"org":..., "complete":..., "items":[...]}`. Reports and
  `account transactions` emit tool-owned flattened shapes with the same
  context header. Source tracking objects within account-transaction rows
  also retain Xero's fields. CLI-owned filter and grouping metadata keeps
  its documented shape; it describes requested context, not a source
  tracking object. `api` emits the raw body.
- Mutations echo the affected entity and print on stderr the
  organisation name and ID, the record ID, and the idempotency key used.
- `--dry-run` prints the request body and target organisation and exits
  0 without calling the API.
- Exit 0 success, 1 application error, 2 usage error. Application error
  codes: `not_found`, `invalid_argument` (including no organisation
  selected, an unknown account, tracking option, or tax type, an
  unbalanced journal), `conflict` (wrong state, two active categories, a
  locked period, a Xero validation refusal with Xero's message),
  `unauthenticated` (missing or unreadable secret file, or a failed
  token request, naming the file and `xero auth status`), `forbidden`
  (organisation mismatch, missing scope or Xero role, named),
  `rate_limited` (with the retry delay), `api` (any other 4xx/5xx with
  Xero's message), `internal`.


## Settled decisions

1. **Command shape.** Plural noun for the collection, singular for one
   entity.
2. **Line input.** Both the `--line` spec and `--file` JSON.
3. **Balance sheet by tracking.** Keep `--by` as a client-side fan-out
   with the non-footing caveat in help.
4. **Tracking category is always explicit.** `tracking=CATEGORY=OPTION`,
   `--tracking CATEGORY=OPTION`, `--by CATEGORY`. No per-organisation
   default in config.
5. **Cash-basis on manual journals is forced.** `add` requires one of
   `--cash-basis` or `--no-cash-basis`.
6. **CSV** on the report commands and `account transactions` only. JSON
   everywhere. Revisit if wrong.
7. **Secrets** live in a file the TOML names per organisation. No tokens
   are stored; one is fetched per run.
8. **Idempotency keys** are generated and printed on every write.
9. **JSON shape.** Native Xero objects with normalised dates and original
   tracking fields for entities; tool-owned flat shapes for reports and
   account transactions. Source tracking objects retain Xero's field names
   without aliases, including within account-transaction rows.
10. **The ledger feed is last.** No command depends on it.
