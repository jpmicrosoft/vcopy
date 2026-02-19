package auth

import (
	"testing"
)

func TestAuthenticate_UnknownMethod(t *testing.T) {
	_, _, err := Authenticate("invalid", "github.com", "github.com", "", "")
	if err == nil {
		t.Fatal("expected error for unknown auth method")
	}
	if err.Error() != "unknown auth method: invalid" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuthenticate_PatWithTokens(t *testing.T) {
	src, tgt, err := Authenticate("pat", "github.com", "ghes.example.com", "src-token", "tgt-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != "src-token" {
		t.Errorf("source token = %q, want %q", src, "src-token")
	}
	if tgt != "tgt-token" {
		t.Errorf("target token = %q, want %q", tgt, "tgt-token")
	}
}

func TestAuthenticate_AutoWithTokens(t *testing.T) {
	src, tgt, err := Authenticate("auto", "github.com", "ghes.example.com", "s", "t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != "s" || tgt != "t" {
		t.Errorf("got (%q, %q), want (%q, %q)", src, tgt, "s", "t")
	}
}

func TestAuthenticateTarget_PatWithToken(t *testing.T) {
	tgt, err := AuthenticateTarget("pat", "ghes.example.com", "my-target-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt != "my-target-token" {
		t.Errorf("target token = %q, want %q", tgt, "my-target-token")
	}
}

func TestAuthenticateTarget_AutoWithToken(t *testing.T) {
	tgt, err := AuthenticateTarget("auto", "github.com", "tgt-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt != "tgt-tok" {
		t.Errorf("target token = %q, want %q", tgt, "tgt-tok")
	}
}

func TestAuthenticateTarget_UnknownMethod(t *testing.T) {
	_, err := AuthenticateTarget("invalid", "github.com", "")
	if err == nil {
		t.Fatal("expected error for unknown auth method")
	}
}
