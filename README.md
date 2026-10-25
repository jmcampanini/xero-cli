# xero

`xero` reads organisation settings, accounts, tracking categories, financial reports, bank transactions, bank transfers, manual journals, contacts, and document attachments from the Xero Accounting API. Each Custom Connection is bound to a verified organisation ID, secrets stay in protected files, tokens are never stored, and the CLI never prompts. The `api` command also accepts explicit write requests.


## Install

Distribution is currently from HEAD only.

```sh
brew tap jmcampanini/xero-cli https://github.com/jmcampanini/xero-cli
brew install --HEAD jmcampanini/xero-cli/xero
brew upgrade --fetch-HEAD jmcampanini/xero-cli/xero
```

To build from source on macOS, install Git and [mise](https://mise.jdx.dev),
clone the repository, and let mise supply the pinned tools from `mise.toml`.
The last step copies the binary onto `PATH`.

```sh
brew install git mise
git clone https://github.com/jmcampanini/xero-cli
cd xero-cli
mise trust
mise install
mise run build
install ./build/xero "$(brew --prefix)/bin/xero"
```


## Representative commands

Start with `xero orgs` and `xero auth status` to confirm which organisation is selected and that its credentials work, then read that organisation's accounts, reports, and listings.

| Command | Result |
| --- | --- |
| `xero orgs` | Configured organisations and the current selection |
| `xero auth status` | Credential, identity, scope, and access diagnostics |
| `xero account show 200 --date 2025-12-31` | Account details for code 200 and its trial-balance amount |
| `xero account transactions 429 --month 2025-12 --tracking Region=North --csv` | Matching cash movements as CSV, without account balances |
| `xero report profit-and-loss --month 2025-12 --basis cash` | Native cash-basis profit and loss |
| `xero report trial-balance --date 2025-12-31 --json` | Debit and credit columns as decimal strings |
| `xero bank-transactions list --account 'Example Bank' --month 2025-12` | Every saved transaction for the bank account and month |
| `xero contacts list --search Example` | Contact names, roles, statuses, and IDs |
| `xero api Organisation` | The raw Accounting API response |

`xero --help`, each command's `--help`, and `xero help exit-codes` are the canonical reference.


## Configuration

The CLI discovers `$XDG_CONFIG_HOME/xero/config.toml`, falling back to `~/.config/xero/config.toml`. See `xero config --help` for the TOML shape, precedence, path resolution, and identity discovery.

```toml
default_org = "acme"

[orgs.acme]
client_id = "00000000000000000000000000000000"
secret_file = "~/.config/xero/secrets/acme"
organisation_id = "00000000-0000-0000-0000-000000000000"
```
