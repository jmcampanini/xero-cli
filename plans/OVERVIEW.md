# xero - Design overview

`xero` is a Go command-line tool over the Xero Accounting API for keeping
the books of one or more small businesses, each in its own Xero
organisation, where transaction lines are tagged with a tracking category
(a region, a department, a project, a property). It does from a terminal
and from scripts what the Xero web UI makes slow: enter and verify bank
transactions, read financial reports broken down by tracking option, look
at the transactions behind any account, post manual journals, and maintain
the chart of accounts and tracking options. It is meant for a person at a
terminal and for an agent in a script, and it never confuses one
organisation with another.

The command spec is `COMMANDS.md`; the research it rests on is
`DOMAIN.md`; delivery order is `MILESTONES.md`.


## Primitives

**Organisation** - one company's Xero account, and a named connection in
config binding one Xero app to its verified organisation ID. Selected by
`--org`, `XERO_ORG`, or `default_org`; never guessed. Every API call
carries its ID and every write names it.

**Account** - a row in the chart of accounts. Addressed by code. Has a
type, class, tax type, status, and for bank accounts a currency.

**Tracking category** and **tracking option** - the two-level dimension
Xero attaches to transaction lines. At most two categories are active per
organisation. Addressed by name, discovered per organisation, never
assumed to be the same across organisations.

**Document** - a bank transaction, a bank transfer, or a manual journal.
Each has lines coded to accounts and optionally tracked, an accounting
date, a status with a small state machine, and a GUID. The two the tool
writes most are the bank transaction (Receive Money, Spend Money) and the
manual journal.

**Line spec** - the tool's one grammar for a document line: account,
amount or debit/credit, description, tax type, tracking as
`Category=Option`, always explicit. The same `--line` flag serves bank
transactions and manual journals; `--file` takes Xero's JSON for anything
beyond it.

**Report** - a read-only projection Xero computes: profit and loss,
balance sheet, trial balance, bank summary. Every report states its
organisation, dates, and basis. Profit and loss can be columned by
tracking category; the balance sheet can only be filtered by option.

**Account transactions** - not a Xero primitive. A tool-assembled view of
every line hitting one account in a date range, built from documents
because Xero has no account-transactions endpoint and its general ledger
feed is restricted to a paid tier.


## Auth

Custom Connections only, for now: Xero's paid, one-organisation,
client-credentials app type. The TOML names each organisation's client
ID, a `secret_file` outside version control, and the organisation ID it
must answer as. Nothing else is stored; every run fetches a 30-minute
token from the secret and checks the organisation against `/connections`
before any accounting call. `auth status` reports what each organisation
can do and what is missing, so a missing scope is diagnosed by name
before it becomes a 401. Browser authorisation is a later option, never a
prerequisite.


## Design rules

- Read before write in every slice. A resource gets `list` and `show`
  before it gets `add`, and `show` is the verification step after every
  write.
- Faithful over formatted. Accounting dates stay `YYYY-MM-DD`, tracking
  is always rendered by name, every report column Xero returns is kept,
  and a list is complete or labelled partial.
- Explicit over defaulted where accounting meaning changes: basis on
  reports, cash-basis treatment on manual journals, organisation when
  more than one is configured, whole months for monthly comparisons,
  the tracking category on every line and filter.
- No prompts. Writes are dry-runnable, idempotent by key, and echo the
  result with the organisation and record ID on stderr.
- Command help is the documentation. Every command states its streams,
  JSON shape, what it never does, and the Xero limitation it works
  around.
- The API's own vocabulary for nouns; plain verbs (`add`, `show`, `edit`,
  `list`, `delete`) plus Xero's state and entry verbs (`post`, `void`,
  `archive`, `receive`, `spend`).
- One thin hand-written client under `internal/xero`: token source,
  organisation header and check, rate limiting, error mapping, paging,
  date parsing. Resource packages build on it. No generated SDK.
- A `xero api` passthrough from the first slice, so nothing is
  unreachable while dedicated commands arrive.
- Nothing depends on the general ledger feed. It is the last thing in the
  plan and only improves a command that already works without it.
