package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"database/sql"
)

// Scopes supported by agent-chat.
var SupportedScopes = []string{"agent:read", "agent:write", "agent:admin"}

// DiscoveryDocument is the OAuth 2.0 Authorization Server Metadata document.
type DiscoveryDocument struct {
	Issuer                        string   `json:"issuer"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	RegistrationEndpoint          string   `json:"registration_endpoint"`
	ScopesSupported               []string `json:"scopes_supported"`
	ResponseTypesSupported        []string `json:"response_types_supported"`
	GrantTypesSupported           []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
}

// Client represents a registered OAuth client.
type Client struct {
	ClientID                string    `json:"client_id"`
	ClientSecret            string    `json:"-"`
	ClientName              string    `json:"client_name"`
	RedirectURIs            []string  `json:"redirect_uris"`
	Scopes                  []string  `json:"-"`
	GrantTypes              []string  `json:"grant_types"`
	ResponseTypes           []string  `json:"response_types"`
	TokenEndpointAuthMethod string    `json:"token_endpoint_auth_method"`
	CreatedAt               time.Time `json:"client_id_issued_at,omitempty"`
}

// Token is an issued opaque access token.
type Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	Scope       string    `json:"scope"`
	ClientID    string    `json:"-"`
	CreatedAt   time.Time `json:"-"`
}

type codeEntry struct {
	Code                string
	ClientID            string
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	ExpiresAt           time.Time
	Used                bool
}

var (
	mu        sync.Mutex
	clients   = map[string]*Client{}
	tokens    = map[string]*Token{}
	codes     = map[string]*codeEntry{}
	defaultDB *sql.DB
)

// SetDB sets the database connection (reserved for future persistence).
func SetDB(db *sql.DB) {
	mu.Lock()
	defer mu.Unlock()
	defaultDB = db
}

// Reset clears in-memory state (primarily for tests).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	clients = map[string]*Client{}
	tokens = map[string]*Token{}
	codes = map[string]*codeEntry{}
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func newSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return "oat_" + hex.EncodeToString(b)
}

func newCode() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func issuerFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{
		"error":             code,
		"error_description": desc,
	})
}

// HandleDiscovery serves /.well-known/oauth-authorization-server.
func HandleDiscovery(w http.ResponseWriter, r *http.Request) {
	issuer := issuerFromRequest(r)
	doc := DiscoveryDocument{
		Issuer:                        issuer,
		AuthorizationEndpoint:         issuer + "/oauth/authorize",
		TokenEndpoint:                 issuer + "/oauth/token",
		RegistrationEndpoint:          issuer + "/oauth/register",
		ScopesSupported:               SupportedScopes,
		ResponseTypesSupported:        []string{"code"},
		GrantTypesSupported:           []string{"authorization_code", "client_credentials"},
		CodeChallengeMethodsSupported: []string{"S256"},
	}
	writeJSON(w, http.StatusOK, doc)
}

type registerRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	Scope                   string   `json:"scope"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// HandleRegister handles POST /oauth/register.
func HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid json")
		return
	}
	if req.ClientName == "" {
		req.ClientName = "unnamed-client"
	}
	if req.TokenEndpointAuthMethod == "" {
		req.TokenEndpointAuthMethod = "client_secret_basic"
	}
	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code", "client_credentials"}
	}
	if len(req.ResponseTypes) == 0 {
		req.ResponseTypes = []string{"code"}
	}

	clientID := newID()
	if clientID == "" {
		writeError(w, http.StatusInternalServerError, "server_error", "id generation failed")
		return
	}

	var clientSecret string
	if req.TokenEndpointAuthMethod != "none" {
		clientSecret = newSecret()
	}

	scopes := SupportedScopes
	if req.Scope != "" {
		scopes = strings.Fields(req.Scope)
	}

	client := &Client{
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		Scopes:                  scopes,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		CreatedAt:               time.Now().UTC(),
	}

	mu.Lock()
	clients[clientID] = client
	mu.Unlock()

	resp := map[string]any{
		"client_id":                  client.ClientID,
		"client_name":                client.ClientName,
		"redirect_uris":              client.RedirectURIs,
		"grant_types":                client.GrantTypes,
		"response_types":             client.ResponseTypes,
		"token_endpoint_auth_method": client.TokenEndpointAuthMethod,
		"scope":                      strings.Join(client.Scopes, " "),
		"client_id_issued_at":        client.CreatedAt.Unix(),
	}
	if clientSecret != "" {
		resp["client_secret"] = clientSecret
	}
	writeJSON(w, http.StatusCreated, resp)
}

// HandleAuthorize handles GET /oauth/authorize (auto-approve).
func HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return
	}
	q := r.URL.Query()
	responseType := q.Get("response_type")
	if responseType != "code" {
		writeError(w, http.StatusBadRequest, "unsupported_response_type", "response_type must be code")
		return
	}
	clientID := q.Get("client_id")
	mu.Lock()
	client, ok := clients[clientID]
	mu.Unlock()
	if !ok || client == nil {
		writeError(w, http.StatusBadRequest, "invalid_client", "unknown client")
		return
	}
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" && len(client.RedirectURIs) > 0 {
		redirectURI = client.RedirectURIs[0]
	}
	if !redirectAllowed(client, redirectURI) {
		writeError(w, http.StatusBadRequest, "invalid_request", "redirect_uri not allowed")
		return
	}

	scope := q.Get("scope")
	if scope == "" {
		scope = strings.Join(client.Scopes, " ")
	}
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	if codeChallengeMethod == "" {
		codeChallengeMethod = "S256"
	}

	code := newCode()
	if code == "" {
		writeError(w, http.StatusInternalServerError, "server_error", "code generation failed")
		return
	}
	entry := &codeEntry{
		Code:                code,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		ExpiresAt:           time.Now().Add(10 * time.Minute).UTC(),
	}
	mu.Lock()
	codes[code] = entry
	mu.Unlock()

	u, err := url.Parse(redirectURI)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid redirect_uri")
		return
	}
	values := u.Query()
	values.Set("code", code)
	if state := q.Get("state"); state != "" {
		values.Set("state", state)
	}
	u.RawQuery = values.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func redirectAllowed(client *Client, redirectURI string) bool {
	if len(client.RedirectURIs) == 0 {
		return true
	}
	for _, u := range client.RedirectURIs {
		if u == redirectURI {
			return true
		}
	}
	return false
}

// HandleToken handles POST /oauth/token.
func HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return
	}

	params, err := parseTokenRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid body")
		return
	}

	grantType := params["grant_type"]
	switch grantType {
	case "client_credentials":
		handleClientCredentials(w, params)
	case "authorization_code":
		handleAuthorizationCode(w, params)
	default:
		writeError(w, http.StatusBadRequest, "unsupported_grant_type", "unsupported grant_type")
	}
}

func parseTokenRequest(r *http.Request) (map[string]string, error) {
	params := map[string]string{}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			return nil, err
		}
		for k, v := range m {
			params[k] = fmt.Sprint(v)
		}
		return params, nil
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	for k := range r.Form {
		params[k] = r.Form.Get(k)
	}
	// Basic auth -> client credentials
	if u, p, ok := r.BasicAuth(); ok {
		if _, exists := params["client_id"]; !exists {
			params["client_id"] = u
		}
		if _, exists := params["client_secret"]; !exists {
			params["client_secret"] = p
		}
	}
	return params, nil
}

func authenticateClient(clientID, clientSecret string) (*Client, bool) {
	mu.Lock()
	defer mu.Unlock()
	client, ok := clients[clientID]
	if !ok || client == nil {
		return nil, false
	}
	if client.TokenEndpointAuthMethod == "none" {
		return client, true
	}
	if client.ClientSecret == "" {
		return nil, false
	}
	if clientSecret != client.ClientSecret {
		return nil, false
	}
	return client, true
}

func handleClientCredentials(w http.ResponseWriter, params map[string]string) {
	clientID := params["client_id"]
	clientSecret := params["client_secret"]
	client, ok := authenticateClient(clientID, clientSecret)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	// client_credentials requires a secret
	if client.TokenEndpointAuthMethod == "none" {
		writeError(w, http.StatusUnauthorized, "invalid_client", "public client cannot use client_credentials")
		return
	}

	scope := params["scope"]
	if scope == "" {
		scope = strings.Join(client.Scopes, " ")
	}

	tok := &Token{
		AccessToken: newToken(),
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       scope,
		ClientID:    clientID,
		CreatedAt:   time.Now().UTC(),
	}
	mu.Lock()
	tokens[tok.AccessToken] = tok
	mu.Unlock()

	writeJSON(w, http.StatusOK, tok)
}

func handleAuthorizationCode(w http.ResponseWriter, params map[string]string) {
	codeVal := params["code"]
	clientID := params["client_id"]
	redirectURI := params["redirect_uri"]
	codeVerifier := params["code_verifier"]

	mu.Lock()
	entry, ok := codes[codeVal]
	mu.Unlock()
	if !ok || entry == nil {
		writeError(w, http.StatusBadRequest, "invalid_grant", "invalid code")
		return
	}
	if entry.Used {
		writeError(w, http.StatusBadRequest, "invalid_grant", "code already used")
		return
	}
	if time.Now().UTC().After(entry.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "invalid_grant", "code expired")
		return
	}
	if entry.ClientID != clientID {
		writeError(w, http.StatusBadRequest, "invalid_grant", "client mismatch")
		return
	}
	if redirectURI != "" && redirectURI != entry.RedirectURI {
		writeError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}

	// client authentication optional
	clientSecret := params["client_secret"]
	if clientSecret != "" || hasClientSecret(clientID) {
		if c, ok := authenticateClient(clientID, clientSecret); !ok {
			// allow public clients without secret
			mu.Lock()
			cl, _ := clients[clientID]
			mu.Unlock()
			if cl == nil || cl.TokenEndpointAuthMethod != "none" {
				writeError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
				return
			}
			_ = c
		}
	}

	// PKCE verification
	if entry.CodeChallenge != "" {
		if !verifyPKCE(codeVerifier, entry.CodeChallenge, entry.CodeChallengeMethod) {
			writeError(w, http.StatusBadRequest, "invalid_grant", "pkce verification failed")
			return
		}
	}

	mu.Lock()
	entry.Used = true
	mu.Unlock()

	tok := &Token{
		AccessToken: newToken(),
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       entry.Scope,
		ClientID:    clientID,
		CreatedAt:   time.Now().UTC(),
	}
	mu.Lock()
	tokens[tok.AccessToken] = tok
	mu.Unlock()

	writeJSON(w, http.StatusOK, tok)
}

func hasClientSecret(clientID string) bool {
	mu.Lock()
	defer mu.Unlock()
	c, ok := clients[clientID]
	return ok && c != nil && c.ClientSecret != ""
}

func verifyPKCE(verifier, challenge, method string) bool {
	if method == "" {
		method = "S256"
	}
	if method == "plain" {
		return verifier == challenge
	}
	if method == "S256" {
		h := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(h[:])
		return computed == challenge
	}
	return false
}

// LookupToken returns a stored token if present.
func LookupToken(token string) (*Token, bool) {
	mu.Lock()
	defer mu.Unlock()
	t, ok := tokens[token]
	if !ok {
		return nil, false
	}
	return t, true
}
