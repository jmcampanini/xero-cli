# Domain map

This page records what the Xero Accounting API exposes, what existing Xero
CLIs and MCP servers chose to surface, and which of those choices matter for
`xero`. It is the evidence behind `COMMANDS.md`. Research date: 2026-09-13.


## Constraints that shape the design

These five facts decide more than any inspiration below.

- **No Go SDK.** `XeroAPI/xerogolang` is archived on OAuth 1.0a. The one
  community client (`entanglesoftware/xero-go`, 2026, generated from the
  OpenAPI spec, zero stars, no release) is not worth depending on. The
  client is a thin layer over `net/http` and `golang.org/x/oauth2`, with
  models transcribed from `xero_accounting.yaml` for the resources we use.
- **The general ledger feed is paywalled.** Since March 2026 the `Journals`
  endpoint needs `accounting.journals.read`, granted only on the Advanced
  tier (about $1,445 AUD a month). Apps and custom connections created
  after 29 April 2026 cannot request it. `ManualJournals` is unaffected.
  The API has never had an Account Transactions report; that request was
  marked "not planned" in 2019. So "show me the transactions on account
  200" has no direct endpoint and must be assembled client-side. Nothing
  in the plan depends on the feed; it is the last item and only improves
  a command that already works.
- **Balance sheet by tracking category is a filter, not a breakdown.**
  Profit and loss accepts `trackingCategoryID` and returns one column per
  option. Balance sheet accepts only `trackingOptionID1` and
  `trackingOptionID2`, so a per-option balance sheet is one call per
  option, and untagged lines (bank accounts, equity) only appear on the
  unfiltered report. Per-option balance sheets will not foot to the org
  total. Trial balance, aged, bank summary, budget summary, and executive
  summary reports take no tracking parameters at all.
- **Bank reconciliation is not in the API.** Xero's bank statements page
  states that unreconciled statement lines and reconciliation are not
  exposed and are not planned. The API has `IsReconciled` on bank
  transactions and the Bank Summary report. Reconciliation stays a web
  UI task.
- **Scopes are granular for new apps.** Apps created after 2 March 2026 get
  `accounting.invoices`, `accounting.payments`,
  `accounting.banktransactions`, `accounting.manualjournals`,
  `accounting.contacts`, `accounting.settings`, `accounting.attachments`,
  `accounting.budgets.read`, and one `accounting.reports.<name>.read` per
  report, each with a `.read` variant where writes exist. Apps created
  before that keep the broad `accounting.transactions`,
  `accounting.settings`, `accounting.contacts`, and
  `accounting.reports.read` scopes until September 2027. A missing scope
  is an HTTP 401 `insufficient_scope`. The tool reports which scopes the
  token carries against the commands that need them.


## Auth story

Xero has three app types. A **Custom Connection** is a paid ($5 USD a
month, bought by the organisation's subscriber, AU/NZ/UK/US organisations
only, free on the Demo Company) client-credentials app bound to exactly
one organisation. A **Mobile or desktop app** is a free public client
using authorization code with PKCE, a loopback redirect, and a browser. A
**Web app** adds a client secret to the browser flow and gains nothing
for a CLI.

Custom Connections are the assumed starting point. Client credentials:
`POST https://identity.xero.com/connect/token` with Basic
`client_id:client_secret` and `grant_type=client_credentials` returns a
30-minute access token and no refresh token. There is nothing to renew;
the client requests a new token on expiry or 401. A custom connection is
bound to one organisation, so `/connections` returns exactly one entry,
which the tool checks against config on every run. This is what makes
"works through ordinary expiry and restarts" trivially true: cache
nothing, fetch a token per run, never ask the user.

Browser authorisation (PKCE on a free Mobile or desktop app, 30-minute
access token, rotating refresh token with a 60-day idle expiry,
`offline_access` required) is the second option for a user without a
Custom Connection. A new app gets only granular scopes and cannot get the
ledger feed. It is a later slice and never a prerequisite.

`GET https://api.xero.com/connections` lists organisations; every
Accounting call needs the `xero-tenant-id` header. Rate limits are 5
concurrent, 60 a minute, and a daily cap per organisation (1,000 on the
free Starter tier, 5,000 on Core), with `Retry-After` on 429. Existing
tools store tokens in the OS keychain, an AES-GCM file, or a plain `0600`
file; two moved from keychain to file because of macOS prompts. This tool
stores no tokens and reads the client secret from a `0600` file the
config names.


## Accounting API resources

Base `https://api.xero.com/api.xro/2.0/`. `PUT` creates, `POST` creates or
updates, and status changes (void, delete, archive, post) are `POST` with a
`Status` field. Paged endpoints take `page` and `pageSize` (max 1,000) and
return a `pagination` block. `where` is a small expression language
(`Status=="AUTHORISED"`, `Date>=DateTime(2025,01,01)`,
`BankAccount.AccountID==Guid("...")`, `Name.Contains("x")`) that cannot
reach line items. `If-Modified-Since` gives incremental reads. JSON dates
arrive as `/Date(1573755038314+0000)/` with an ISO `DateString` beside them
on most documents.

| Resource | Read | Write | State changes | Notes |
| --- | --- | --- | --- | --- |
| Accounts | list, by id | create, update, delete (unused only) | archive | no paging; `where`, `order` |
| BankTransactions | list, by id | create, update | delete | spend, receive, transfers, over/prepayments; paged |
| BankTransfers | list, by id | create | delete | no edits |
| BatchPayments | list, by id | create | delete | |
| BrandingThemes | list, by id | none | | |
| Budgets | list, by id | none | | carry `Tracking`; `DateFrom`, `DateTo` |
| Contacts | list, by id, by number | create, update | archive | `searchTerm`, `summaryOnly`, paged |
| ContactGroups | list, by id | create, update, add/remove contacts | delete | |
| CreditNotes | list, by id, pdf | create, update, allocate | approve, void, delete | paged |
| Currencies | list | create | | |
| ExpenseClaims, Receipts | list, by id | create, update | | deprecated classic expenses |
| History | per resource | add note | | audit trail |
| Invoices (ACCREC sales, ACCPAY bills) | list, by id, by number, pdf, online url | create, update, email | approve, void, delete | `Statuses`, `ContactIDs`, `InvoiceNumbers`, `searchTerm`, paged |
| Items | list, by id | create, update, delete | | |
| Journals | list (offset by number), by id, by number | none | | Advanced tier only; 100 per call; `paymentsOnly` |
| LinkedTransactions | list, by id | create, update, delete | | billable expenses |
| ManualJournals | list, by id | create, update | post, void, delete | lines carry `Tracking`; paged |
| Organisation | get, actions | none | | what the app may do |
| Overpayments, Prepayments | list, by id | allocate | | created via BankTransactions |
| Payments | list, by id | create | delete | paged |
| PaymentServices | list | create | | certification needed |
| PurchaseOrders | list, by id, by number, pdf | create, update | approve, bill, delete | `DateFrom`, `DateTo`, no `where` |
| Quotes | list, by id, pdf | create, update | send, accept, decline, delete | no `where` |
| RepeatingInvoices | list, by id | create, update | delete | |
| Reports | see below | none | | |
| TaxRates | list, by type | create, update | delete | |
| TrackingCategories | list, by id | create, rename, delete | archive | at most 2 active, 100 options each |
| TrackingOptions | via category | add, rename, delete | archive | |
| Users | list, by id | none | | |
| Attachments | list, download | upload | | on most documents; 10 per document, 25 MB |

Other Xero APIs (Files, Assets, Payroll, Projects, Bank Feeds, Finance) are
out of scope. Finance has the closest thing to account transactions
(`BankStatementsPlus`) but is restricted to lending partners.


## Reports

| Report | Parameters | Tracking |
| --- | --- | --- |
| BalanceSheet | `date`, `periods` 1-11, `timeframe` MONTH/QUARTER/YEAR, `standardLayout`, `paymentsOnly` | filter: `trackingOptionID1`, `trackingOptionID2` |
| ProfitAndLoss | `fromDate`, `toDate`, `periods` 1-11, `timeframe`, `standardLayout`, `paymentsOnly` | columns: `trackingCategoryID` (+ `trackingOptionID`), `trackingCategoryID2` (+ `trackingOptionID2`); 1,000-column cap |
| TrialBalance | `date`, `paymentsOnly` | none |
| AgedReceivablesByContact, AgedPayablesByContact | `contactId` (required), `date`, `fromDate`, `toDate` | none; org-wide aged means one call per contact |
| BankSummary | `fromDate`, `toDate` | none |
| BudgetSummary | `date`, `periods` 1-12, `timeframe` as 1/3/12 | none |
| ExecutiveSummary | `date` | none |
| TenNinetyNine | `reportYear` | US only, different shape |
| GST/BAS | list then by `ReportID` | AU/NZ only |

Every report except 1099 returns `Reports[0]` with `ReportTitles`, a
`Header` row whose cells are column captions, then `Section` rows holding
`Row` and `SummaryRow` children. Cells are strings; account rows carry an
`Attributes` entry with `Id: "account"` and the AccountID. The tool owns a
flattening step (section, label, account id, one value per column) so
`--json` and `--csv` output is stable.

Reports, Journals, and ManualJournals return 403 when the authorising Xero
user lacks the Reports role.


## Account transactions without the ledger feed

The web UI's Account Transactions report is not in the API. Options by
fidelity:

1. `Journals` filtered on `JournalLines[].AccountID`, running balance
   seeded from the trial balance. Complete. Needs the tier-restricted
   scope, so it is the last item in the plan and an optional path the
   tool may use when the token carries it.
2. Bank accounts: union of `BankTransactions` where
   `BankAccount.AccountID`, `Payments` where `Account.AccountID`,
   `BankTransfers` on either side, plus `Prepayments` and `Overpayments`,
   ordered by date. Complete for bank accounts.
3. Other accounts: scan `Invoices`, `CreditNotes`, `BankTransactions`, and
   `ManualJournals` over the date range with `page` set (so line items
   come back) and filter `LineItems[].AccountCode` client-side, since
   `where` cannot target line items. Use `If-Modified-Since` for
   incremental caching. This misses system-generated lines (FX, tax,
   payroll) that only Journals carry.
4. Balances only: trial balance at a date, or profit and loss with tracking
   for per-option totals.

Issue #5 narrows the first implementation to cash-only, base-currency bank
transactions, transfers and cash-enabled manual journals. It excludes invoice
and bill payment reconstruction, refunds, and prepayment/overpayment
allocations. BankTransactions records for prepayments and overpayments remain
visible where returned, but their later allocations/refunds are not rebuilt.
Every output discloses incomplete coverage, including for bank accounts.
Tracking filters show period movements rather than reconstructed balances.
It is the most expensive read command in the tool.


## The bookkeeping this serves

- One or more small businesses, each in its own Xero organisation, each
  with at least one tracking category whose options segment the reports
  (a region, a department, a project, a property). Structures differ
  between organisations and are discovered, never assumed.
- Monthly entry is one Receive Money per customer and one Spend Money per
  supplier in a bank account, each with several lines to different
  accounts and tracking options, quantity 1, an explicit tax type, and an
  amount mode that may vary. Movements between the business's own bank
  accounts are transfers, never income or expense.
- After every save the record is reopened and every field compared:
  contact, accounting date, description, account, tracking option,
  quantity, tax, amount. Formatted output from at least one existing tool
  dropped tracking values and shifted midnight dates to a local time
  zone, so the tool must preserve accounting dates as `YYYY-MM-DD` and
  always render tracking names.
- Selecting a contact in Xero applies that contact's default tax type to
  a new transaction. Every saved line must show tax type and tax amount
  so the default can be checked, and `tax=NONE` must be expressible.
- Duplicate avoidance means listing the bank account for the month by
  contact, reference, and amount before entering, and excluding deleted
  records.
- Year-end work is manual journals with tracking per line: accruals,
  prepayment releases, corrections, depreciation and amortisation.
- The reporting loop is cash and accrual profit and loss for exact months
  and years, profit and loss by tracking option, balance sheets, trial
  balances, and monthly bank cash movements per account, each matched to
  Xero's own report. A cash profit and loss must never be presented as a
  cash-flow statement.
- A small business is a few hundred bank transactions a year, so
  full-year lists are a few pages, well inside daily limits.


## Existing tools

Fifteen Xero CLIs and about nineteen MCP servers with verifiable tool
lists were surveyed. The ones that matter:

- **Official CLI** `@xeroapi/xero-command-line` (TypeScript, oclif, alpha
  v0.0.7, March 2026). Noun-verb, PKCE, named profiles each bound to one
  organisation, `--json`/`--csv`/`--toon`. No `get`, no delete or void,
  no `--where`, no tracking filter on reports, no per-call organisation
  override. Every write takes `--file` JSON. Ships a `SKILL.md` for
  agents.
- **paulmeller/xero-cli** (Go, cobra, active August 2026). The widest verb
  set (void, email, pdf, attach, allocate, history) and the only tool with
  `--tracking-category-id` and `--tracking-option-id` on reports. Generic
  resource descriptor, `sync` to JSONL or DuckDB, `organisation actions`,
  `rate-limits`, exit codes mapped from Xero errors, `--no-prompt` when
  stdin is not a terminal.
- **osodevops/xero-cli** (Rust, dormant since April 2026). 34 nouns
  mapping 1:1 to endpoints, scope presets, SQLite response cache with
  `--no-cache`, aged reports fan out per contact automatically.
- **cjw296/xerotrust** (Python, PyPI). Read-only `export` of every endpoint
  to dated JSONL, plus `check` and `reconcile` over the export.
- **inscoder/xero-cli** (Go). Splits `invoices` and `bills` into separate
  nouns owning ACCREC and ACCPAY. Strict JSON input validation.
- **msmithstubbs/xero-cli** (Go, modelled on `gh`). `describe` prints a
  machine-readable command schema; `--fields` projection; `--redact` by
  default.
- **marcinbogdanski/xero-cli**. No nouns; `invoke <api> <method>`
  passthrough with a per-method allow/ask/block policy and audit log.
- **Official MCP server** `XeroAPI/xero-mcp-server` (51 tools). Client
  credentials or a bearer token you supply; always `connections[0]`.
  Exposes tracking category and option writes, manual journal
  create/update without tracking on lines (a `TODO` in source), balance
  sheet with `trackingOptionID1/2`, profit and loss with tracking hardcoded
  to `undefined`, and a five-argument call that misroutes `paymentsOnly`.
  No Journals, no `get-invoice`, no void.
- **Hosted `mcp.xero.com`** is read-only, undocumented, and scoped to
  invoices, settings, aged, balance sheet, and profit and loss.
- **derekclair/xero-mcp** (56 tools) and **mrsmickers/xero-mcp** are the
  only servers with both tracking dimensions on both profit and loss and
  balance sheet.


## Noun frequency

Counts over 14 CLIs with explicit nouns and 19 MCP servers. "Common" means
in more than half of each population.

| Noun | CLIs /14 | MCPs /19 | Tier |
| --- | --- | --- | --- |
| Invoices (and bills) | 12 | 19 | common |
| Contacts | 11 | 19 | common |
| Accounts (chart) | 12 | 18 | common (reads); writes in 5 CLIs, 2 MCPs |
| Organisation, tenant selection | 6 | 17 | common |
| Payments | 9 | 17 | common |
| Bank transactions | 9 | 16 | common |
| Report: profit and loss | 6 | 14 | common |
| Report: balance sheet | 5 | 14 | common |
| Credit notes | 7 | 12 | majority |
| Tax rates | 6 | 12 | majority (read-only everywhere) |
| Report: trial balance | 5 | 12 | majority |
| Report: aged receivables / payables | 5 / 4 | 12 | majority |
| Items | 6 | 11 | majority |
| Tracking categories and options | 5 | 11 | majority (writes in 3 CLIs, 5 MCPs) |
| Quotes | 5 | 10 | majority |
| Manual journals | 5 | 9 | half (writes in 3 CLIs, 8 MCPs) |
| Currencies | 5 | 3 | split |
| Bank transfers | 5 | 1 | split |
| Attachments (upload, download) | 3 | 6 | minority |
| Report: executive summary | 2 | 6 | minority |
| Journals (general ledger) | 3 | 5 | minority, read-only everywhere |
| Contact groups | 3 | 5 | minority, read-only in MCPs |
| Purchase orders | 4 | 5 | minority |
| Report: bank summary, budget summary | 2 | 5 | minority |
| Overpayments, prepayments | 4 | 1 | rare |
| Repeating invoices | 4 | 3 | rare |
| Batch payments | 3 | 1 | rare |
| Budgets | 1 | 4 | rare, read-only |
| Branding themes | 3 | 1 | rare, read-only |
| Invoice PDF | 3 | 2 | rare |
| History and notes | 2 | 1 | rare |
| Linked transactions | 2 | 1 | rare |
| Users, employees | 2 | 1 | rare, read-only |
| Local export or sync store | 4 | 0 | rare, CLI-only idea |
| Response cache | 2 | 0 | rare, CLI-only idea |
| API passthrough | 2 | 2 | rare, high leverage |
| Receipts, expense claims, payment services | 1 | 1 | rare, deprecated endpoints |
| Report: GST/BAS, 1099 | 1 | 0 | rare, regional |
| Invoice email | 1 | 4 | rare |
| Rate limits | 1 | 0 | rare |
| Payroll, Projects, Files, Assets, Finance | 0 | 1-5 | out of scope |


## Verb frequency

| Verb | CLIs /14 | MCPs (any resource) | Notes |
| --- | --- | --- | --- |
| list | 14 | 19 | universal |
| login / auth, logout, status | 11 / 8 / 8 | n/a | |
| get one by id | 8 | 12 | official CLI has none; uses list filters |
| create | 8 | 16 | |
| update | 7 | 14 | |
| tenant list / switch | 5 | 5 | |
| config show / set | 4 | 0 | |
| export / sync | 4 | 0 | |
| refresh token | 4 | 0 | |
| delete | 3 | 6 | rare; usually a status change |
| archive | 3 | 1 | accounts, contacts, tracking |
| attach / upload | 3 | 5 | |
| pdf | 3 | 2 | |
| void | 1 | 4 | plus two bulk-void scripts |
| approve / authorise | 1 | 1 | others do it via `update --status` |
| allocate (credit, over, prepayment to invoice) | 2 | 1 | |
| history | 2 | 1 | |
| email | 1 | 4 | |
| online-url | 2 | 2 | |
| post (manual journal) | 0 | 0 | everyone does it via status on create/update |
| describe / schema | 1 | 3 | agent-oriented |
| api passthrough | 2 | 2 | |

Capability flags worth keeping: `--where` and `--order` raw filters (5
CLIs), `--modified-since` (5), `--all` auto-pagination (2), per-call
organisation override (5), `--dry-run` on writes (4), `--idempotency-key`
(3), stdin JSON via `--file -` (3), `--fields` projection (3). Output
formats: JSON everywhere, CSV in 3 CLIs, TSV, YAML, TOON, JSONL once each.


## Grammar observations

- Every modern tool is noun-verb. The only option-style or verb-noun
  designs are from 2017 and 2018.
- Nouns are kebab-case (`credit-notes`, `bank-transactions`,
  `manual-journals`) in every maintained tool.
- Tracking is modelled three ways: nested `tracking categories|options`
  (official), `tracking ... options add` (paulmeller), and flat
  `tracking-categories add-option` (osodevops). None address categories
  or options by name; all take GUIDs.
- Reports are always a `reports` group with one subcommand per Xero report
  ID. As-at reports take `--date`; range reports take `--from`/`--to` or
  `--from-date`/`--to-date`.
- Single-record reads are a `get ID` verb in most tools and a
  `list --invoice-id` filter in the official CLI and inscoder.
- Multi-organisation handling is either profile-bound (one organisation
  per profile, chosen at login), a switchable current organisation with
  a per-call override, or a required per-call flag. The official MCP
  server always uses the first connection.
- Nearly every 2026 tool ships agent guidance (`SKILL.md`, `describe`,
  `--toon`) and a wrong-organisation guard before writes.


## Ideas to borrow and ideas to reject

Borrow:

- `--tracking-category` and `--tracking-option` on reports (paulmeller),
  addressed by name instead of GUID.
- An API passthrough with the same auth, output, and error handling as
  every other command (marcinbogdanski, venetanji, `gh api`).
- `organisation actions` and the rate-limit headers as a `status` surface
  (paulmeller).
- Exit codes mapped from Xero error classes (paulmeller, inscoder).
- Confirm-the-organisation-before-writing guidance in help and a
  `--dry-run` on writes.
- Idempotency keys on every write.

Reject:

- Named profiles bound to organisations (official). An organisation is a
  config value with an env and flag override, not a profile layer.
- A local response cache or sync store in v1. Read commands hit the API;
  a cache is a later decision once daily limits bite.
- `--toon`, YAML, TSV. JSON everywhere, CSV on reports.
- Aliases (`tx`, `inv`), sugar verbs, `update --status VOIDED` as the way
  to void. One spelling per action.
- Ever selecting `connections[0]` without a configured organisation ID.
- Invoicing workflows, deprecated endpoints, payroll, projects, files,
  assets.
