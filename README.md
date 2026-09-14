# xero

`xero` reads organisation settings, accounts, and tracking categories from the Xero Accounting API. Each Custom Connection is bound to a verified organisation ID, secrets stay in protected files, and tokens are never stored. The `api` command also accepts explicit write requests.

Command help is the reference. Start with `xero --help`, `xero config --help`, and `xero help exit-codes`.

## Install

Distribution is currently from HEAD only.

```sh
brew tap jmcampanini/xero-cli https://github.com/jmcampanini/xero-cli
brew install --HEAD jmcampanini/xero-cli/xero
brew upgrade --fetch-HEAD jmcampanini/xero-cli/xero
```

To build from source with Go 1.27.1:

```sh
make build
./build/xero --version
```

## Representative commands

| Command | Result |
| --- | --- |
| `xero config --provenance` | Effective TOML and the source of each value |
| `xero orgs` | Configured organisations and the current selection |
| `xero auth status` | Credential, identity, scope, and access diagnostics |
| `xero org show` | The selected organisation's settings |
| `xero accounts list --bank` | Active bank accounts |
| `xero account show 200` | Account details for code 200, when configured |
| `xero tracking-categories list --all` | Active and archived categories |
| `xero tracking-category show Property` | The named category and all options |
| `xero api Organisation` | The raw Accounting API response |

## Required external programs

None. The installed CLI does not run external programs or prompt.

## Configuration

The CLI discovers `$XDG_CONFIG_HOME/xero/config.toml`, falling back to `~/.config/xero/config.toml`. `--config PATH` replaces discovery and requires an existing file. Each organisation names a Custom Connection client ID, an organisation ID, and a secret file with no group or other permission bits. See `xero config --help` for the TOML shape, precedence, path resolution, and identity discovery.
