// Package xero implements Custom Connections and the Xero Accounting API.
// Accounting dates use their UTC calendar day, never the local timezone.
// UpdatedDateUTC and CreatedDateUTC are RFC 3339 UTC timestamps.
package xero

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"
)

// Options supplies a connection's credentials and injectable HTTP boundaries.
type Options struct {
	BaseURL        string
	Budget         *Budget
	ClientID       string
	ConnectionsURL string
	HTTPClient     *http.Client
	Name           string
	OrganisationID string
	Scopes         []string
	SecretFile     string
	TokenURL       string
	Version        string
}

// Budget shares concurrency and request limits across organisations in one process.
type Budget struct {
	limiter *rate.Limiter
	slots   chan struct{}
}

// NewBudget permits at most five concurrent requests and sixty per minute.
func NewBudget() *Budget {
	return &Budget{limiter: rate.NewLimiter(rate.Every(time.Second), 1), slots: make(chan struct{}, 5)}
}

// Client owns one connection's in-memory token and verified organisation identity.
type Client struct {
	budget     *Budget
	http       *http.Client
	identityMu sync.Mutex
	limits     http.Header
	limitsMu   sync.Mutex
	options    Options
	tenantID   string
	token      *oauth2.Token
	tokenMu    sync.Mutex
}

// New constructs a client without reading files or making requests.
func New(options Options) *Client {
	if options.BaseURL == "" {
		options.BaseURL = "https://api.xero.com/api.xro/2.0/"
	}
	if options.ConnectionsURL == "" {
		options.ConnectionsURL = "https://api.xero.com/connections"
	}
	if options.TokenURL == "" {
		options.TokenURL = "https://identity.xero.com/connect/token"
	}
	if options.Budget == nil {
		options.Budget = NewBudget()
	}
	httpClient := http.Client{Timeout: 30 * time.Second}
	if options.HTTPClient != nil {
		httpClient = *options.HTTPClient
	}
	// Never follow redirects with credentials or request bodies to another endpoint.
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{budget: options.Budget, http: &httpClient, options: options, limits: make(http.Header)}
}

func (c *Client) accessToken(ctx context.Context, force bool) (*oauth2.Token, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if !force && c.token.Valid() {
		return c.token, nil
	}
	secret, err := config.ReadSecret(c.options.SecretFile)
	if err != nil {
		return nil, err
	}
	credentials := clientcredentials.Config{ClientID: c.options.ClientID, ClientSecret: secret, TokenURL: c.options.TokenURL, Scopes: c.options.Scopes, AuthStyle: oauth2.AuthStyleInHeader}
	token, err := credentials.Token(context.WithValue(ctx, oauth2.HTTPClient, c.http))
	if err != nil {
		var response *oauth2.RetrieveError
		if errors.As(err, &response) {
			code := response.ErrorCode
			if code == "" {
				code = "token_request_refused"
			}
			key := "client_id or secret_file"
			if code == "invalid_scope" {
				key = "scopes (must match the developer portal)"
			}
			return nil, apperr.New("unauthenticated", "organisation %s: OAuth %s; fix orgs.%s.%s", c.options.Name, code, c.options.Name, key)
		}
		return nil, apperr.New("unauthenticated", "organisation %s: token request failed; check orgs.%s.client_id, secret_file and network access", c.options.Name, c.options.Name)
	}
	c.token = token
	return token, nil
}

// Connection identifies the organisation returned by Xero's connections endpoint.
type Connection struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenantId"`
	TenantName string `json:"tenantName"`
	TenantType string `json:"tenantType"`
}

func (c *Client) connections(ctx context.Context) ([]Connection, error) {
	_, _, body, err := c.request(ctx, Request{Method: http.MethodGet, Path: c.options.ConnectionsURL}, "")
	if err != nil {
		return nil, err
	}
	var connections []Connection
	if err := json.Unmarshal(body, &connections); err != nil {
		return nil, apperr.New("api", "decode connections: %v", err)
	}
	return connections, nil
}

func (c *Client) verify(ctx context.Context) (string, error) {
	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	if c.tenantID != "" {
		return c.tenantID, nil
	}
	if c.options.OrganisationID == "" {
		return "", apperr.New("invalid_argument", "orgs.%s.organisation_id is not set; run 'xero auth status %s' and copy the ID it prints", c.options.Name, c.options.Name)
	}
	connections, err := c.connections(ctx)
	if err != nil {
		return "", err
	}
	connection, err := c.matchConnection(connections)
	if err != nil {
		return "", err
	}
	c.tenantID = connection.TenantID
	return c.tenantID, nil
}

func (c *Client) matchConnection(connections []Connection) (Connection, error) {
	var matches []Connection
	var answers []string
	for _, connection := range connections {
		answers = append(answers, connection.TenantID+" ("+connection.TenantName+")")
		if strings.EqualFold(connection.TenantID, c.options.OrganisationID) {
			matches = append(matches, connection)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return Connection{}, apperr.New("forbidden", "organisation %s is configured as ID %s but the connection answers for %s; fix organisation_id or the client ID", c.options.Name, c.options.OrganisationID, strings.Join(answers, ", "))
}

// Request describes a raw Accounting API call, including optional conditional headers.
type Request struct {
	Body    []byte
	Headers http.Header
	Method  string
	Path    string
	Query   url.Values
}

// Do sends a raw request after verifying the configured organisation.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body []byte) (int, http.Header, []byte, error) {
	return c.DoRequest(ctx, Request{Body: body, Method: method, Path: path, Query: query})
}

// DoRequest sends a raw request with optional headers after verifying identity.
func (c *Client) DoRequest(ctx context.Context, request Request) (int, http.Header, []byte, error) {
	if _, err := c.resolveURL(request.Path, request.Query); err != nil {
		return 0, nil, nil, err
	}
	tenantID, err := c.verify(ctx)
	if err != nil {
		return 0, nil, nil, err
	}
	return c.request(ctx, request, tenantID)
}

func (c *Client) resolveURL(path string, query url.Values) (string, error) {
	parsed, err := url.Parse(path)
	if err != nil {
		return "", apperr.New("invalid_argument", "invalid API path %q", path)
	}
	if parsed.IsAbs() {
		if path != c.options.ConnectionsURL {
			return "", apperr.New("invalid_argument", "absolute API path must be https://api.xero.com/connections")
		}
	} else {
		if parsed.Host != "" || strings.HasPrefix(path, "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", apperr.New("invalid_argument", "API path must be relative; use --param for query parameters")
		}
		for _, segment := range strings.Split(parsed.Path, "/") {
			if segment == ".." || segment == "." {
				return "", apperr.New("invalid_argument", "API path cannot contain . or .. segments")
			}
		}
		base, baseErr := url.Parse(c.options.BaseURL)
		if baseErr != nil {
			return "", apperr.New("internal", "invalid Accounting API base URL")
		}
		parsed = base.ResolveReference(parsed)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) request(ctx context.Context, request Request, tenantID string) (int, http.Header, []byte, error) {
	endpoint, err := c.resolveURL(request.Path, request.Query)
	if err != nil {
		return 0, nil, nil, err
	}
	refetched, retried := false, false
	for {
		token, err := c.accessToken(ctx, false)
		if err != nil {
			return 0, nil, nil, err
		}
		req, err := http.NewRequestWithContext(ctx, request.Method, endpoint, bytes.NewReader(request.Body))
		if err != nil {
			return 0, nil, nil, apperr.New("invalid_argument", "create API request: %v", err)
		}
		for key, values := range request.Headers {
			req.Header[key] = append([]string(nil), values...)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "xero-cli/"+c.options.Version)
		if tenantID != "" {
			req.Header.Set("xero-tenant-id", tenantID)
		}
		if request.Body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		status, headers, body, err := c.send(ctx, req)
		if err != nil {
			return status, headers, body, err
		}
		if status == http.StatusUnauthorized && !refetched {
			refetched = true
			if _, err := c.accessToken(ctx, true); err != nil {
				return status, headers, body, err
			}
			continue
		}
		if status == http.StatusTooManyRequests && !retried {
			retried = true
			if err := wait(ctx, retryDelay(headers)); err != nil {
				return status, headers, body, err
			}
			continue
		}
		if status < 200 || status >= 300 {
			return status, headers, body, responseError(status, headers, body)
		}
		return status, headers, body, nil
	}
}

func (c *Client) send(ctx context.Context, request *http.Request) (int, http.Header, []byte, error) {
	if err := c.budget.limiter.Wait(ctx); err != nil {
		return 0, nil, nil, apperr.New("api", "wait for request budget: %v", err)
	}
	select {
	case c.budget.slots <- struct{}{}:
	case <-ctx.Done():
		return 0, nil, nil, apperr.New("api", "request canceled: %v", ctx.Err())
	}
	defer func() { <-c.budget.slots }()
	response, err := c.http.Do(request)
	if err != nil {
		return 0, nil, nil, apperr.New("api", "send Xero request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	c.limitsMu.Lock()
	for _, key := range []string{"X-DayLimit-Remaining", "X-MinLimit-Remaining", "X-AppMinLimit-Remaining"} {
		if value := response.Header.Get(key); value != "" {
			c.limits.Set(key, value)
		}
	}
	c.limitsMu.Unlock()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, response.Header, nil, apperr.New("api", "read Xero response: %v", err)
	}
	return response.StatusCode, response.Header, body, nil
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return apperr.New("api", "request canceled: %v", ctx.Err())
	}
}

// Limits returns the most recently received rate-limit values, omitting unknown values.
func (c *Client) Limits() http.Header {
	c.limitsMu.Lock()
	defer c.limitsMu.Unlock()
	return c.limits.Clone()
}

// Identity is the configured alias and verified organisation ID used in list output.
type Identity struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Identity returns the selected connection's configured identity.
func (c *Client) Identity() Identity {
	return Identity{Name: c.options.Name, ID: c.options.OrganisationID}
}

func decodeResource[T any](body []byte, key string) ([]T, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, apperr.New("api", "decode %s: %v", key, err)
	}
	resource, ok := envelope[key]
	if !ok {
		return nil, apperr.New("api", "Xero response has no %s array", key)
	}
	var items []T
	if err := json.Unmarshal(resource, &items); err != nil {
		return nil, apperr.New("api", "decode %s: %v", key, err)
	}
	if items == nil {
		items = []T{}
	}
	return items, nil
}

func requireOne[T any](items []T, name string) (T, error) {
	if len(items) == 1 {
		return items[0], nil
	}
	var zero T
	return zero, apperr.New("not_found", "%s: expected one result, received %d", name, len(items))
}
