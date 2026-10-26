# xero

`xero` reads organisation settings, accounts, tracking categories, financial reports, bank transactions, bank transfers, manual journals, contacts, and document attachments from the Xero Accounting API. Each Custom Connection is bound to a verified organisation ID, secrets stay in protected files, tokens are never stored, and the CLI never prompts. The `api` command also accepts explicit write requests.


## Install

Until the first versioned release is published, install from HEAD or build from source. The release workflow will provide prebuilt binaries for Linux and macOS on amd64 and arm64; no first-release version has been selected.

After the first stable release, install and upgrade with [mise](https://mise.jdx.dev) or the shared Homebrew tap:

```sh
mise use -g github:jmcampanini/xero-cli
mise upgrade github:jmcampanini/xero-cli
brew install --cask jmcampanini/tap/xero
brew upgrade --cask jmcampanini/tap/xero
```

Alternatively, download the matching `xero_<version>_<os>_<arch>.tar.gz` from the [releases page](https://github.com/jmcampanini/xero-cli/releases), extract it, and copy `xero` onto your `PATH`. Use `darwin` for macOS or `linux` for Linux, with `amd64` or `arm64` for the architecture. To upgrade, download the new archive and replace the binary. Releases include a SHA-256 checksums file and GitHub build provenance attestations. If macOS quarantines a browser download, remove the attribute with `xattr -d com.apple.quarantine xero` before running it.

The existing HEAD formula remains available as an alternative and includes shell completions. Keep using its fully qualified name to install or upgrade from the latest source:

```sh
brew tap jmcampanini/xero-cli https://github.com/jmcampanini/xero-cli
brew install --HEAD jmcampanini/xero-cli/xero
brew upgrade --fetch-HEAD jmcampanini/xero-cli/xero
```

Both Homebrew packages install the `xero` executable and bash, zsh, and fish completions. When switching from the HEAD formula to the release cask, first run `brew uninstall jmcampanini/xero-cli/xero` to avoid an executable conflict.

To build from source, install Git and [mise](https://mise.jdx.dev) (`brew install git mise` on macOS), clone the repository, and let mise supply the pinned tools. The last step copies the binary into `~/.local/bin`; add that directory to `PATH` if your shell does not already include it.

```sh
git clone https://github.com/jmcampanini/xero-cli
cd xero-cli
mise trust
mise install
mise run build
install -d ~/.local/bin
install ./build/xero ~/.local/bin/xero
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
| `xero bank-transactions list --account 'Example Bank' --month 2025-12` | Saved transactions for the bank account and month, excluding deleted records |
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
