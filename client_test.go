package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestResolveAuth_TokenFromFlag(t *testing.T) {
	auth, err := resolveAuth("https://mm.example.com", "my-token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Token != "my-token" {
		t.Errorf("expected token 'my-token', got %q", auth.Token)
	}
	if auth.ServerURL != "https://mm.example.com" {
		t.Errorf("expected URL 'https://mm.example.com', got %q", auth.ServerURL)
	}
}

func TestResolveAuth_TokenFromEnv(t *testing.T) {
	t.Setenv("MM_TOKEN", "env-token")
	auth, err := resolveAuth("https://mm.example.com", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Token != "env-token" {
		t.Errorf("expected token 'env-token', got %q", auth.Token)
	}
}

func TestResolveAuth_URLFromEnv(t *testing.T) {
	t.Setenv("MM_URL", "https://env.example.com/")
	auth, err := resolveAuth("", "tok", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.ServerURL != "https://env.example.com" {
		t.Errorf("expected trailing slash stripped, got %q", auth.ServerURL)
	}
}

func TestResolveAuth_NoURL(t *testing.T) {
	t.Setenv("MM_URL", "")
	_, err := resolveAuth("", "tok", "")
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitErr.Code)
	}
}

func TestResolveAuth_NoAuth(t *testing.T) {
	t.Setenv("MM_TOKEN", "")
	t.Setenv("MM_USERNAME", "")
	_, err := resolveAuth("https://mm.example.com", "", "")
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitErr.Code)
	}
}

func TestResolveAuth_UsernameFromEnv(t *testing.T) {
	t.Setenv("MM_USERNAME", "admin")
	t.Setenv("MM_PASSWORD", "secret")
	// MM_PASSWORD is checked before the interactive prompt, so this always works.
	auth, err := resolveAuth("https://mm.example.com", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", auth.Username)
	}
	if auth.Password != "secret" {
		t.Errorf("expected password 'secret', got %q", auth.Password)
	}
}

func TestResolveAuth_UsernameFromFlag(t *testing.T) {
	t.Setenv("MM_USERNAME", "env-admin")
	t.Setenv("MM_PASSWORD", "secret")
	auth, err := resolveAuth("https://mm.example.com", "", "flag-admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Username != "flag-admin" {
		t.Errorf("expected flag username to take precedence, got %q", auth.Username)
	}
}

func TestResolveAuth_PasswordEnvTakesPrecedenceOverPrompt(t *testing.T) {
	t.Setenv("MM_PASSWORD", "env-password")
	password, err := obtainPassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if password != "env-password" {
		t.Errorf("expected MM_PASSWORD to be used, got %q", password)
	}
}

func TestResolveAuth_NoPasswordAvailable(t *testing.T) {
	t.Setenv("MM_PASSWORD", "")
	// stdin in tests is not a terminal, so this should fail.
	_, err := obtainPassword()
	if err == nil {
		t.Fatal("expected error when no password available")
	}
}

func TestResolveAuth_FlagPrecedenceOverEnv(t *testing.T) {
	t.Setenv("MM_TOKEN", "env-token")
	auth, err := resolveAuth("https://mm.example.com", "flag-token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Token != "flag-token" {
		t.Errorf("expected flag token to take precedence, got %q", auth.Token)
	}
}

func TestResolveAuth_TokenPrecedenceOverUsername(t *testing.T) {
	auth, err := resolveAuth("https://mm.example.com", "my-token", "my-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Token != "my-token" {
		t.Errorf("expected token auth to take precedence, got token=%q", auth.Token)
	}
	if auth.Username != "" {
		t.Errorf("expected empty username when token is used, got %q", auth.Username)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"first non-empty", []string{"a", "b"}, "a"},
		{"skip empty", []string{"", "b"}, "b"},
		{"all empty", []string{"", ""}, ""},
		{"single value", []string{"x"}, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.values...)
			if got != tt.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestWrapAPIError_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantCode   int
		wantSubstr string
	}{
		{"401 unauthorized", 401, ExitConfigError, "authentication failed"},
		{"403 forbidden", 403, ExitConfigError, "permission denied"},
		{"404 not found", 404, ExitConfigError, "not found"},
		{"500 server error", 500, ExitAPIError, "unexpected error (HTTP 500)"},
		{"502 bad gateway", 502, ExitAPIError, "unexpected error (HTTP 502)"},
		{"400 bad request", 400, ExitAPIError, "unexpected API response (HTTP 400)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &model.Response{StatusCode: tt.statusCode}
			exitErr := wrapAPIError(resp, fmt.Errorf("api error"), "test resource")
			if exitErr.Code != tt.wantCode {
				t.Errorf("expected exit code %d, got %d", tt.wantCode, exitErr.Code)
			}
			if !strings.Contains(exitErr.Message, tt.wantSubstr) {
				t.Errorf("expected message to contain %q, got %q", tt.wantSubstr, exitErr.Message)
			}
		})
	}
}

func TestWrapAPIError_NilResponse(t *testing.T) {
	exitErr := wrapAPIError(nil, fmt.Errorf("connection refused"), "test")
	if exitErr.Code != ExitAPIError {
		t.Errorf("expected exit code %d, got %d", ExitAPIError, exitErr.Code)
	}
	if !strings.Contains(exitErr.Message, "unable to connect") {
		t.Errorf("expected connection error message, got %q", exitErr.Message)
	}
}
