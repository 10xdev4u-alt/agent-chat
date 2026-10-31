CREATE TABLE IF NOT EXISTS oauth_clients (
    client_id              TEXT PRIMARY KEY,
    client_secret          TEXT,
    client_name            TEXT,
    redirect_uris          TEXT,
    scopes                 TEXT,
    grant_types            TEXT,
    response_types         TEXT,
    token_endpoint_auth_method TEXT,
    created_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    access_token  TEXT PRIMARY KEY,
    token_type    TEXT NOT NULL DEFAULT 'Bearer',
    client_id     TEXT,
    subject       TEXT,
    scope         TEXT,
    expires_at    TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS oauth_codes (
    code                    TEXT PRIMARY KEY,
    client_id               TEXT NOT NULL,
    redirect_uri            TEXT,
    scope                   TEXT,
    code_challenge          TEXT,
    code_challenge_method   TEXT,
    expires_at              TEXT,
    used                    INTEGER NOT NULL DEFAULT 0,
    created_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_client ON oauth_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_codes_client ON oauth_codes(client_id);
