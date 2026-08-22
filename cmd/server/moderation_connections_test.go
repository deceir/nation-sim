package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestClientIPPrefersRailwayHeader(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:44000"
	r.Header.Set("X-Real-IP", "203.0.113.42")
	if got := requestClientIP(r); got != "203.0.113.42" {
		t.Fatalf("requestClientIP() = %q", got)
	}
}

func TestConnectionTokenIsStableAndOpaque(t *testing.T) {
	secret := []byte("test-only-secret")
	first := connectionToken(secret, "203.0.113.42")
	second := connectionToken(secret, "203.0.113.42")
	other := connectionToken(secret, "203.0.113.43")
	if first == "" || first != second || first == other {
		t.Fatalf("connection tokens are not stable and distinct")
	}
	if strings.Contains(first, "203.0.113.42") || len(first) != 64 {
		t.Fatalf("connection token is not opaque: %q", first)
	}
	publicID := publicConnectionID(secret, first)
	if len(publicID) != 20 || strings.Contains(publicID, first) {
		t.Fatalf("public connection ID is not appropriately separated: %q", publicID)
	}
}
