package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
)

type contextKey struct{}

var agentIDKey = contextKey{}

type BearerToken string

func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return "bus_" + hex.EncodeToString(b), nil
}

func HashKey(key string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash key: %w", err)
	}
	return string(h), nil
}

func ValidateKey(key, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)) == nil
}

func BearerTokenFromHeader(header string) (BearerToken, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 {
		return "", false
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	if parts[1] == "" {
		return "", false
	}
	return BearerToken(parts[1]), true
}

var defaultDB *sql.DB

func SetDB(db *sql.DB) {
	defaultDB = db
}

func Middleware(next http.Handler) http.Handler {
	return MiddlewareWithDB(defaultDB, next)
}

func MiddlewareWithDB(dbConn *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := BearerTokenFromHeader(r.Header.Get("Authorization"))
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent-chat"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if dbConn == nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent-chat"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		agent, err := dbpkg.GetAgentByKey(dbConn, string(token))
		if err != nil || agent == nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent-chat"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), agentIDKey, agent.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AgentIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(agentIDKey).(string)
	return v, ok
}
