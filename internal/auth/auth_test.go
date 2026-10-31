package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if !strings.HasPrefix(key, "bus_") {
		t.Fatalf("expected bus_ prefix, got %s", key)
	}
	// bus_ + 64 hex chars
	if len(key) != 4+64 {
		t.Fatalf("unexpected key length: %d", len(key))
	}
	key2, _ := GenerateAPIKey()
	if key == key2 {
		t.Fatal("keys should differ")
	}
}

func TestHashAndValidateKey(t *testing.T) {
	key, _ := GenerateAPIKey()
	hash, err := HashKey(key)
	if err != nil {
		t.Fatalf("HashKey: %v", err)
	}
	if !ValidateKey(key, hash) {
		t.Fatal("ValidateKey should be true")
	}
	if ValidateKey("wrong", hash) {
		t.Fatal("ValidateKey should be false for wrong key")
	}
}

func TestBearerTokenFromHeader(t *testing.T) {
	tok, ok := BearerTokenFromHeader("Bearer abc123")
	if !ok || tok != "abc123" {
		t.Fatalf("got %v %v", tok, ok)
	}
	if _, ok := BearerTokenFromHeader("Basic x"); ok {
		t.Fatal("expected false")
	}
	if _, ok := BearerTokenFromHeader(""); ok {
		t.Fatal("expected false")
	}
}
