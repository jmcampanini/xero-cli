package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/spf13/pflag"
)

func configFlags(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("config", "", "")
	if err := pflagloader.Register[Config](flags); err != nil {
		t.Fatal(err)
	}
	if err := flags.Parse(args); err != nil {
		t.Fatal(err)
	}
	return flags
}

func fixtureConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `default_org = "acme"
[orgs.acme]
client_id = "00000000000000000000000000000000"
secret_file = "secrets/acme"
organisation_id = "acme-id"
[orgs.beta]
client_id = "11111111111111111111111111111111"
secret_file = "~/secrets/beta"
organisation_id = "beta-id"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadPrecedenceAndRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := fixtureConfig(t)
	for _, tc := range []struct{ name, env, flag, want, source string }{
		{"file", "", "", "acme", path},
		{"environment", "beta", "", "beta", configloader.SourceEnv},
		{"flag", "beta", "acme", "acme", pflagloader.SourcePFlag},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XERO_ORG", tc.env)
			if tc.env == "" {
				if err := os.Unsetenv("XERO_ORG"); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"--config", path}
			if tc.flag != "" {
				args = append(args, "--org", tc.flag)
			}
			cfg, report, err := Load(configFlags(t, args...), path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Org != tc.want || report.Updates["org"] != tc.source {
				t.Errorf("Load org/source = %q/%q, want %q/%q", cfg.Org, report.Updates["org"], tc.want, tc.source)
			}
			if cfg.Orgs["acme"].SecretFile != filepath.Join(filepath.Dir(path), "secrets/acme") {
				t.Errorf("relative secret = %q", cfg.Orgs["acme"].SecretFile)
			}
			for _, provenance := range []bool{false, true} {
				body, err := TOML(cfg, report, provenance)
				if err != nil {
					t.Fatal(err)
				}
				roundtrip := filepath.Join(t.TempDir(), "config.toml")
				if err := os.WriteFile(roundtrip, body, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Unsetenv("XERO_ORG"); err != nil {
					t.Fatal(err)
				}
				loaded, _, err := Load(configFlags(t, "--config", roundtrip), roundtrip)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(loaded, cfg) {
					t.Errorf("round-trip = %#v, want %#v", loaded, cfg)
				}
				if provenance && tc.flag != "" && !strings.Contains(string(body), "# flag: --org") {
					t.Errorf("missing flag provenance: %s", body)
				}
			}
		})
	}
}

func TestDiscoveryAndMissingExplicitConfig(t *testing.T) {
	t.Setenv("XERO_ORG", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, _, err := Load(configFlags(t), "")
	if err != nil || len(cfg.Orgs) != 0 {
		t.Fatalf("optional discovery = %#v, %v", cfg, err)
	}
	missing := filepath.Join(t.TempDir(), "missing.toml")
	_, _, err = Load(configFlags(t, "--config", missing), missing)
	if err == nil || apperr.From(err).Code != "invalid_argument" {
		t.Fatalf("explicit missing file = %v", err)
	}
}

func TestValidationAndSelection(t *testing.T) {
	valid := OrgConfig{ClientID: strings.Repeat("a", 32), SecretFile: "secret"}
	for _, tc := range []struct {
		name           string
		cfg            Config
		selected, code string
	}{
		{"zero", Config{}, "", "invalid_argument"},
		{"one", Config{Orgs: map[string]OrgConfig{"acme": valid}}, "acme", ""},
		{"two", Config{Orgs: map[string]OrgConfig{"acme": valid, "beta": valid}}, "", "invalid_argument"},
		{"selected", Config{Org: "beta", Orgs: map[string]OrgConfig{"acme": valid, "beta": valid}}, "beta", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.cfg.Effective()
			if got != tc.selected {
				t.Errorf("Effective = %q, want %q", got, tc.selected)
			}
			if tc.code != "" && (err == nil || apperr.From(err).Code != tc.code) || tc.code == "" && err != nil {
				t.Errorf("Effective error = %v, want %q", err, tc.code)
			}
		})
	}
	for _, name := range []string{"UPPER", "two--hyphens", "-first", "last-", "has space", "../escape"} {
		err := (Config{Orgs: map[string]OrgConfig{name: valid}}).Validate()
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("Validate name %q = %v", name, err)
		}
	}
	for _, org := range []OrgConfig{{ClientID: "short", SecretFile: "file"}, {ClientID: valid.ClientID}} {
		if err := (Config{Orgs: map[string]OrgConfig{"acme": org}}).Validate(); err == nil {
			t.Error("invalid org accepted")
		}
	}
	if err := (Config{Org: "unknown"}).Validate(); err == nil {
		t.Error("unknown default accepted")
	}
}

func TestSecretPathsAndPermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, tc := range []struct{ path, source, want string }{
		{"~/secret", "", filepath.Join(home, "secret")},
		{"secret", filepath.Join(home, "config.toml"), filepath.Join(home, "secret")},
		{filepath.Join(home, "secret"), "", filepath.Join(home, "secret")},
	} {
		got, err := ResolveSecretPath(tc.path, tc.source)
		if err != nil || got != tc.want {
			t.Errorf("ResolveSecretPath = %q, %v, want %q", got, err, tc.want)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ResolveSecretPath("relative", configloader.SourceEnv)
	if err != nil || got != filepath.Join(cwd, "relative") {
		t.Errorf("env-relative = %q, %v", got, err)
	}
	path := filepath.Join(home, "secret")
	for _, tc := range []struct {
		name, content string
		mode          os.FileMode
		want          string
	}{
		{"private", " \nsecret-value\t", 0o600, "secret-value"},
		{"owner-read", "secret-value", 0o400, "secret-value"},
		{"public", "secret-value", 0o644, ""},
		{"empty", " \n", 0o600, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}
			secret, err := ReadSecret(path)
			if tc.want != "" {
				if err != nil || secret != tc.want {
					t.Errorf("ReadSecret = %q, %v", secret, err)
				}
			} else if err == nil || apperr.From(err).Code != "unauthenticated" {
				t.Errorf("ReadSecret = %v, want unauthenticated", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-value") {
				t.Error("secret leaked in error")
			}
			if err := os.Chmod(path, 0o600); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, path := range []string{home, filepath.Join(home, "absent")} {
		if _, err := ReadSecret(path); err == nil {
			t.Errorf("ReadSecret(%q) unexpectedly succeeded", path)
		}
	}
}
