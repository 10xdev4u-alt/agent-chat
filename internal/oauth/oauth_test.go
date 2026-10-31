package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDiscovery(t *testing.T) {
	Reset()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()
	HandleDiscovery(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var doc DiscoveryDocument
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Issuer != "http://example.com" {
		t.Fatalf("issuer %s", doc.Issuer)
	}
	if doc.TokenEndpoint != "http://example.com/oauth/token" {
		t.Fatalf("token endpoint %s", doc.TokenEndpoint)
	}
	if len(doc.ScopesSupported) == 0 {
		t.Fatal("no scopes")
	}
}

func TestRegisterAndClientCredentials(t *testing.T) {
	Reset()
	body := `{"client_name":"test","redirect_uris":["http://localhost/cb"],"scope":"agent:read agent:write"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	HandleRegister(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status %d: %s", w.Code, w.Body.String())
	}
	var reg map[string]any
	json.Unmarshal(w.Body.Bytes(), &reg)
	clientID := reg["client_id"].(string)
	clientSecret := reg["client_secret"].(string)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", "agent:read")
	req2 := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	HandleToken(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("token status %d: %s", w2.Code, w2.Body.String())
	}
	var tok Token
	json.Unmarshal(w2.Body.Bytes(), &tok)
	if tok.AccessToken == "" || tok.TokenType != "Bearer" {
		t.Fatalf("bad token: %+v", tok)
	}
	if _, ok := LookupToken(tok.AccessToken); !ok {
		t.Fatal("token not stored")
	}
}

func TestAuthorizationCodePKCE(t *testing.T) {
	Reset()
	body := `{"client_name":"spa","redirect_uris":["http://localhost/cb"],"token_endpoint_auth_method":"none"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	HandleRegister(w, req)
	var reg map[string]any
	json.Unmarshal(w.Body.Bytes(), &reg)
	clientID := reg["client_id"].(string)

	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", "http://localhost/cb")
	q.Set("scope", "agent:read")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", "xyz")
	req2 := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+q.Encode(), nil)
	w2 := httptest.NewRecorder()
	HandleAuthorize(w2, req2)
	if w2.Code != http.StatusFound {
		t.Fatalf("authorize status %d: %s", w2.Code, w2.Body.String())
	}
	loc := w2.Header().Get("Location")
	u, _ := url.Parse(loc)
	code := u.Query().Get("code")
	if code == "" {
		t.Fatal("no code in redirect")
	}
	if u.Query().Get("state") != "xyz" {
		t.Fatal("state mismatch")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", "http://localhost/cb")
	form.Set("client_id", clientID)
	form.Set("code_verifier", verifier)
	req3 := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w3 := httptest.NewRecorder()
	HandleToken(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("token status %d: %s", w3.Code, w3.Body.String())
	}
	var tok Token
	json.Unmarshal(w3.Body.Bytes(), &tok)
	if tok.AccessToken == "" {
		t.Fatal("no access token")
	}

	form2 := url.Values{}
	form2.Set("grant_type", "authorization_code")
	form2.Set("code", code)
	form2.Set("redirect_uri", "http://localhost/cb")
	form2.Set("client_id", clientID)
	form2.Set("code_verifier", verifier)
	req4 := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form2.Encode()))
	req4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w4 := httptest.NewRecorder()
	HandleToken(w4, req4)
	if w4.Code == http.StatusOK {
		t.Fatal("replay should fail")
	}
}
