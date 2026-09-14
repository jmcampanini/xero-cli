package xero

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/config"
)

// AuthStatus reports independent diagnostics without exposing the token or secret.
type AuthStatus struct {
	Name             string `json:"name"`
	ClientID         string `json:"client_id"`
	SecretFile       string `json:"secret_file"`
	Secret           string `json:"secret"`
	SecretOK         bool   `json:"secret_ok"`
	Token            string `json:"token"`
	TokenOK          bool   `json:"token_ok"`
	OrganisationID   string `json:"organisation_id"`
	OrganisationName string `json:"organisation_name"`
	Organisation     string `json:"organisation"`
	OrganisationOK   bool   `json:"organisation_ok"`
	Scopes           string `json:"scopes"`
	Commands         string `json:"commands"`
	Limits           string `json:"limits"`
}

// AuthStatus inspects credentials and identity, including when the configured ID is absent.
func (c *Client) AuthStatus(ctx context.Context) (AuthStatus, error) {
	status := AuthStatus{Name: c.options.Name, ClientID: c.options.ClientID, SecretFile: c.options.SecretFile, Secret: "ok", Token: "unavailable", Organisation: "unavailable", Scopes: "unavailable", Commands: "unavailable", Limits: "unavailable"}
	if _, err := config.ReadSecret(c.options.SecretFile); err != nil {
		status.Secret = err.Error()
		return status, err
	}
	status.SecretOK = true
	token, err := c.accessToken(ctx, false)
	if err != nil {
		status.Token = err.Error()
		return status, err
	}
	status.TokenOK = true
	status.Token = "ok"
	scopes, expiry, firstErr := tokenClaims(token.AccessToken)
	if firstErr == nil {
		status.Token = fmt.Sprintf("ok, expires in %s", time.Until(expiry).Round(time.Minute))
		status.Scopes = strings.Join(scopes, " ")
		capabilities := Capabilities(scopes)
		var commands []string
		for _, group := range []string{"accounts", "tracking", "reports", "transactions", "contacts"} {
			state := "ok"
			if !capabilities[group] {
				state = "missing (" + requiredScope(group) + ")"
			}
			commands = append(commands, group+" "+state)
		}
		status.Commands = strings.Join(commands, " · ")
		if !capabilities["accounts"] || !capabilities["tracking"] {
			firstErr = apperr.New("forbidden", "organisation %s: accounts and tracking require accounting.settings.read or accounting.settings; update the Custom Connection in the developer portal", c.options.Name)
		}
	} else {
		status.Token = firstErr.Error()
		status.TokenOK = false
	}

	connections, connectionErr := c.connections(ctx)
	if connectionErr != nil {
		status.Organisation = connectionErr.Error()
	} else {
		if len(connections) == 1 {
			status.OrganisationID = connections[0].TenantID
			status.OrganisationName = connections[0].TenantName
		}
		if c.options.OrganisationID == "" {
			if len(connections) == 1 {
				status.Organisation = "not set in config (copy this ID)"
			} else {
				connectionErr = apperr.New("forbidden", "organisation %s: Custom Connection returned %d organisations; expected one", c.options.Name, len(connections))
				status.Organisation = connectionErr.Error()
			}
		} else {
			connection, matchErr := c.matchConnection(connections)
			connectionErr = matchErr
			if matchErr != nil {
				status.Organisation = "MISMATCH: config says " + c.options.OrganisationID
			} else {
				status.OrganisationID = connection.TenantID
				status.OrganisationName = connection.TenantName
				status.Organisation = "matches config"
				status.OrganisationOK = true
			}
		}
	}
	if firstErr == nil {
		firstErr = connectionErr
	}
	limits := c.Limits()
	var rows []string
	for _, entry := range [][2]string{{"day", "X-DayLimit-Remaining"}, {"minute", "X-MinLimit-Remaining"}, {"app minute", "X-AppMinLimit-Remaining"}} {
		value := limits.Get(entry[1])
		if value == "" {
			value = "unavailable"
		} else {
			value += " remaining"
		}
		rows = append(rows, entry[0]+" "+value)
	}
	status.Limits = strings.Join(rows, " · ")
	return status, firstErr
}

func requiredScope(group string) string {
	switch group {
	case "accounts", "tracking":
		return "accounting.settings.read"
	case "reports":
		return "accounting.reports.read or accounting.reports.<name>.read"
	case "transactions":
		return "accounting.transactions.read or banktransactions/manualjournals.read"
	case "contacts":
		return "accounting.contacts.read"
	default:
		return "unknown"
	}
}

func tokenClaims(token string) ([]string, time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, time.Time{}, apperr.New("unauthenticated", "access token has no readable JWT claims")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, time.Time{}, apperr.New("unauthenticated", "access token has invalid JWT encoding")
	}
	var claims struct {
		Exp   int64           `json:"exp"`
		Scope json.RawMessage `json:"scope"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return nil, time.Time{}, apperr.New("unauthenticated", "access token has invalid scope/exp claims")
	}
	var scopes []string
	if err := json.Unmarshal(claims.Scope, &scopes); err != nil {
		var text string
		if err := json.Unmarshal(claims.Scope, &text); err != nil {
			return nil, time.Time{}, apperr.New("unauthenticated", "access token has invalid scope claims")
		}
		scopes = strings.Fields(text)
	}
	return scopes, time.Unix(claims.Exp, 0).UTC(), nil
}
