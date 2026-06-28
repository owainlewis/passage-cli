package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunShowsHelpByDefault(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(nil, &stdout, &stderr, BuildInfo{Version: "test"})

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunLoginSavesConfigAndRedactsToken(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := t.TempDir()

	code := RunWithRuntime([]string{"login"}, Runtime{
		Stdin:     strings.NewReader("http://localhost:8080/\npsg_abcdefghijklmnopqrstuvwxyz\n"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		ConfigDir: dir,
		Build:     BuildInfo{Version: "test"},
	})

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("stdout leaked token: %s", stdout.String())
	}
	statusOut := bytes.Buffer{}
	statusErr := bytes.Buffer{}
	code = RunWithRuntime([]string{"auth", "status"}, Runtime{
		Stdout:    &statusOut,
		Stderr:    &statusErr,
		ConfigDir: dir,
		Env:       map[string]string{},
		Build:     BuildInfo{Version: "test"},
	})
	if code != 0 {
		t.Fatalf("status code = %d, stderr = %s", code, statusErr.String())
	}
	if !strings.Contains(statusOut.String(), "API URL: http://localhost:8080 (config)") {
		t.Fatalf("status stdout = %s", statusOut.String())
	}
	if strings.Contains(statusOut.String(), "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("status leaked token: %s", statusOut.String())
	}
}

func TestRunAuthStatusUsesEnvOverrides(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := RunWithRuntime([]string{"auth", "status"}, Runtime{
		Stdout:    &stdout,
		Stderr:    &stderr,
		ConfigDir: t.TempDir(),
		Env: map[string]string{
			"PASSAGE_API_URL": "http://localhost:8080",
			"PASSAGE_TOKEN":   "psg_envtoken",
		},
		Build: BuildInfo{Version: "test"},
	})

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "API URL: http://localhost:8080 (env)") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Token: psg_...") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "envtoken") {
		t.Fatalf("stdout leaked token: %s", stdout.String())
	}
}

func TestRunAuthStatusCheckCallsServer(t *testing.T) {
	var sawAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"authenticated":true,"user":{"id":"user-1","email":"u@example.com"}}`)
	}))
	defer server.Close()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := RunWithRuntime([]string{"auth", "status", "--check"}, Runtime{
		Stdout:    &stdout,
		Stderr:    &stderr,
		ConfigDir: t.TempDir(),
		Env: map[string]string{
			"PASSAGE_API_URL": server.URL,
			"PASSAGE_TOKEN":   "psg_checktoken",
		},
		HTTP:  server.Client(),
		Build: BuildInfo{Version: "test"},
	})

	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if sawAuth != "Bearer psg_checktoken" {
		t.Fatalf("authorization = %q", sawAuth)
	}
	if !strings.Contains(stdout.String(), "Server: authenticated as u@example.com") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "checktoken") {
		t.Fatalf("stdout leaked token: %s", stdout.String())
	}
}

func TestRunAuthStatusCheckReportsServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := RunWithRuntime([]string{"auth", "status", "--check"}, Runtime{
		Stdout:    &stdout,
		Stderr:    &stderr,
		ConfigDir: t.TempDir(),
		Env: map[string]string{
			"PASSAGE_API_URL": server.URL,
			"PASSAGE_TOKEN":   "psg_badtoken",
		},
		HTTP:  server.Client(),
		Build: BuildInfo{Version: "test"},
	})

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "auth check failed") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestRunAuthStatusFailsWithoutToken(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := RunWithRuntime([]string{"auth", "status"}, Runtime{
		Stdout:    &stdout,
		Stderr:    &stderr,
		ConfigDir: t.TempDir(),
		Env:       map[string]string{},
		Build:     BuildInfo{Version: "test"},
	})

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Not authenticated") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestRunShowsVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr, BuildInfo{
		Version: "v0.1.0",
		Commit:  "abc123",
		Date:    "2026-06-28T00:00:00Z",
	})

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	for _, want := range []string{"passage v0.1.0", "commit abc123", "built 2026-06-28T00:00:00Z"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want to contain %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"wat"}, &stdout, &stderr, BuildInfo{Version: "test"})

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "wat"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
