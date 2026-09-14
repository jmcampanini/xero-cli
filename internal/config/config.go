// Package config owns configuration loading, provenance and organisation selection.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/spf13/pflag"
)

// Config contains named Custom Connections and the selected organisation.
type Config struct {
	Org  string               `toml:"default_org" config:"org" help:"organisation name to use"`
	Orgs map[string]OrgConfig `toml:"orgs"`
}

// OrgConfig binds a client credential to the organisation it must answer for.
type OrgConfig struct {
	ClientID       string   `toml:"client_id"`
	SecretFile     string   `toml:"secret_file"`
	OrganisationID string   `toml:"organisation_id"`
	Scopes         []string `toml:"scopes"`
}

func defaults() Config { return Config{} }

// Load applies file, environment and flag overrides, then validates and resolves paths.
func Load(flags *pflag.FlagSet, explicitPath string) (Config, configloader.LoadReport, error) {
	var fileLoader configloader.ConfigLoader[Config]
	var err error
	if flags.Changed("config") {
		fileLoader, err = configloader.NewRequiredFileLoader[Config](explicitPath)
	} else {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return Config{}, configloader.LoadReport{}, apperr.New("invalid_argument", "resolve config home: %v", homeErr)
			}
			base = filepath.Join(home, ".config")
		}
		fileLoader, err = configloader.NewMergeAllFilesLoader[Config](configloader.File(filepath.Join(base, "xero", "config.toml")))
	}
	if err != nil {
		return Config{}, configloader.LoadReport{}, apperr.New("invalid_argument", "load config: %v", err)
	}
	envLoader, err := configloader.NewEnvironmentLoader[Config]("xero", configloader.OSEnv())
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}
	cfg, report, err := configloader.Load(defaults(), fileLoader, envLoader, flagLoader)
	if err != nil {
		return Config{}, configloader.LoadReport{}, apperr.New("invalid_argument", "load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, configloader.LoadReport{}, err
	}
	for name, org := range cfg.Orgs {
		source := report.Updates["orgs["+strconv.Quote(name)+"].secretfile"]
		org.SecretFile, err = ResolveSecretPath(org.SecretFile, source)
		if err != nil {
			return Config{}, configloader.LoadReport{}, err
		}
		if org.Scopes == nil {
			org.Scopes = []string{}
		}
		cfg.Orgs[name] = org
	}
	return cfg, report, nil
}

// Validate checks identifiers without reading secrets or requiring an organisation selection.
func (c Config) Validate() error {
	namePattern := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	clientPattern := regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
	for _, name := range c.Names() {
		org := c.Orgs[name]
		if !namePattern.MatchString(name) {
			return apperr.New("invalid_argument", "orgs.%s: organisation names must contain lowercase letters, digits and single hyphens", name)
		}
		if !clientPattern.MatchString(org.ClientID) {
			return apperr.New("invalid_argument", "orgs.%s.client_id must contain 32 hexadecimal characters", name)
		}
		if org.SecretFile == "" {
			return apperr.New("invalid_argument", "orgs.%s.secret_file is required", name)
		}
	}
	if c.Org != "" {
		if _, ok := c.Orgs[c.Org]; !ok {
			return apperr.New("invalid_argument", "default_org/--org: organisation %q is not configured", c.Org)
		}
	}
	return nil
}

// Names returns configured names in deterministic order.
func (c Config) Names() []string {
	names := make([]string, 0, len(c.Orgs))
	for name := range c.Orgs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Effective selects the explicit/default organisation or the sole configured one.
func (c Config) Effective() (string, error) {
	if c.Org != "" {
		if _, ok := c.Orgs[c.Org]; ok {
			return c.Org, nil
		}
		return "", apperr.New("invalid_argument", "organisation %q is not configured", c.Org)
	}
	names := c.Names()
	if len(names) == 1 {
		return names[0], nil
	}
	return "", apperr.New("invalid_argument", "no organisation selected; use --org, XERO_ORG, or default_org (configured: %s)", strings.Join(names, ", "))
}

// RequireOrganisationID rejects incomplete identity setup before command input or HTTP work.
func RequireOrganisationID(name, id string) error {
	if id == "" {
		return apperr.New("invalid_argument", "orgs.%s.organisation_id is not set; run 'xero auth status %s' and copy the ID it prints", name, name)
	}
	return nil
}

// ResolveSecretPath expands ~/ and resolves relative paths against their source file.
func ResolveSecretPath(path, source string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", apperr.New("invalid_argument", "resolve secret_file home: %v", err)
		}
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) && filepath.IsAbs(source) {
		path = filepath.Join(filepath.Dir(source), path)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", apperr.New("invalid_argument", "resolve secret_file %q: %v", path, err)
	}
	return resolved, nil
}

// ReadSecret reads only regular owner-private files and never includes their content in errors.
func ReadSecret(path string) (string, error) {
	before, err := os.Stat(path)
	if err != nil {
		return "", apperr.New("unauthenticated", "secret file %q: %v", path, err)
	}
	if !before.Mode().IsRegular() {
		return "", apperr.New("unauthenticated", "secret file %q must be a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", apperr.New("unauthenticated", "secret file %q: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", apperr.New("unauthenticated", "secret file %q: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", apperr.New("unauthenticated", "secret file %q must be a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", apperr.New("unauthenticated", "secret file %q: mode %04o, must be 0600 or stricter", path, info.Mode().Perm())
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return "", apperr.New("unauthenticated", "read secret file %q: %v", path, err)
	}
	secret := strings.TrimSpace(string(content))
	if secret == "" {
		return "", apperr.New("unauthenticated", "secret file %q is empty", path)
	}
	return secret, nil
}

// TOML renders redirectable configuration, optionally annotating each value's source.
func TOML(cfg Config, report configloader.LoadReport, provenance bool) ([]byte, error) {
	encoded, err := configreporter.New(cfg, report).TOML()
	if err != nil {
		return nil, fmt.Errorf("render config: %w", err)
	}
	// Child tables declare their parent implicitly; retain the documented shape.
	encoded = []byte(strings.ReplaceAll(string(encoded), "[orgs]\n", ""))
	if !provenance {
		return encoded, nil
	}
	lines := strings.Split(string(encoded), "\n")
	table := ""
	fields := map[string]string{"client_id": "clientid", "secret_file": "secretfile", "organisation_id": "organisationid", "scopes": "scopes"}
	for i, line := range lines {
		if strings.HasPrefix(line, "[orgs.") {
			table = strings.TrimSuffix(strings.TrimPrefix(line, "[orgs."), "]")
		}
		key, _, ok := strings.Cut(line, " = ")
		if !ok {
			continue
		}
		path := "org"
		if table != "" {
			path = "orgs[" + strconv.Quote(table) + "]." + fields[key]
		}
		source := report.Updates[path]
		switch source {
		case "", configloader.SourceDefault:
			source = "default"
		case configloader.SourceEnv:
			source = "env: XERO_ORG"
		case pflagloader.SourcePFlag:
			source = "flag: --org"
		default:
			source = "file: " + source
		}
		lines[i] += " # " + source
	}
	return []byte(strings.Join(lines, "\n")), nil
}
