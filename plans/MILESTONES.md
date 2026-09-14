# Milestone sketch

Vertical slices, read before write, each ending with a command a person
can run against a real organisation. The first slice includes the
repository scaffold but is not only a scaffold: it authenticates and reads
real data, so it is proven against Xero like every later one.

Write slices need two things: write scopes on the Custom Connection, and a
test organisation (Xero's Demo Company is free) for demonstration writes.
Slices 1 to 4 work with read-only scopes.


## Round 1 - Access and trust


### 1 - Access, chart, and tracking

Repository files (root `main.go`, `cmd/`, `config`, the `exit-codes`
help topic, guard tests, `make check`, the check workflow, a Homebrew
formula, agent instructions), plus `auth status`, `orgs`, `org show`,
`xero api`, `accounts list`, `account show`, `tracking-categories list`,
and `tracking-category show`. Client credentials from the TOML-named
secret file, a token per run, the organisation identity check, `--org`
selection with no fallback, rate limiting, error mapping, exit codes,
secret redaction, name-to-ID resolution for tracking. Proof: each
configured organisation reads its own identity, chart, and tracking
structure across restarts and after a token would have expired; a missing
secret file, a wrong secret, and a missing scope each fail by name.


## Round 2 - Understanding the books


### 2 - Reports

`report profit-and-loss`, `report balance-sheet`, `report trial-balance`,
`report bank-summary` with `--basis`, `--month`, `--year`, `--periods`,
`--tracking`, and `--by` by name, the flattened JSON shape with its
context header, `--csv`, and the balance-sheet fan-out. Proof: monthly
and yearly profit and loss on both bases, profit and loss by tracking
option, and bank summaries match the same reports in the Xero web UI for
representative months.


### 3 - Transactions, read

`manual-journals list`, `manual-journal show`, `bank-transactions list`,
`bank-transaction show`, `bank-transfers list`, `bank-transfer show`,
`contacts list`, `contact show`, and attachments list and download. Full
pagination by default, `--page`, `--where`, `--modified-since`,
`--month`. Proof: a full year of one bank account lists completely with
deleted records marked, and a record entered in the web UI reads back
with every field a bookkeeper checks.


### 4 - Account transactions

`account transactions` assembled from slice 3's reads, with the running
balance, `--csv`, and the documented gap. The most expensive read.


## Round 3 - Controlled bookkeeping


### 5 - Manual journals, write

`manual-journals add`, `manual-journal edit`, `post`, `void`, `delete`,
with the line spec grammar, `--file`, `--dry-run`, idempotency keys,
balance validation, forced `--cash-basis`, and state checks. Proof in a
test organisation: a multi-line, multi-option draft round-trips every
field through `show`; an edit leaves untouched fields intact; posting
changes the trial balance as expected; a retry with the same idempotency
key creates nothing.


### 6 - Receive and Spend Money, write

`bank-transactions receive` and `spend`, `bank-transaction edit` and
`delete`, `bank-transfers add`, `contacts add`, `contact edit`,
`contact archive`. Proof: one month of receipts and payments reproduced
in the test organisation and verified line by line through `show`.


### 7 - Chart and tracking, write

`accounts add`, `account edit`, `account archive`, and every
`tracking-categories` / `tracking-options` mutation.


## Later

- Browser authorisation (`auth login`) for a user without a Custom
  Connection.
- `report bank-summary --periods N --timeframe month`: Xero's bank
  summary has no period columns, so the tool would run one call per
  whole month ending at the range end and lay the columns side by side
  per bank account (opening, received, spent, closing for each month).
- `--csv` on list commands, history, a response cache, export,
  `describe`, relative and financial-year date forms.
- The general ledger feed, last: `journals list`, `journal show`, and
  `account transactions` switching to the feed when the organisation's
  app carries `accounting.journals.read`. Nothing earlier depends on it,
  and most apps cannot get the scope.
