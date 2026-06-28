package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/owainlewis/passage-cli/internal/config"
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

func TestRunDocumentCommands(t *testing.T) {
	dir := t.TempDir()
	if err := config.Save(dir, config.Config{APIURL: "http://passage.test", Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer psg_test" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docs":
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input["body"] != "# Draft\n" {
				t.Fatalf("create body = %q", input["body"])
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":"11111111-1111-1111-1111-111111111111","title":"Draft","body":"# Draft\n","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docs":
			_, _ = io.WriteString(w, `{"documents":[{"id":"11111111-1111-1111-1111-111111111111","title":"Draft","body":"# Draft\n","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docs/11111111-1111-1111-1111-111111111111":
			_, _ = io.WriteString(w, `{"id":"11111111-1111-1111-1111-111111111111","title":"Draft","body":"# Draft","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/docs/11111111-1111-1111-1111-111111111111":
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(input["body"], "More") && !strings.Contains(input["body"], "Pushed") && !strings.Contains(input["body"], "Replaced") {
				t.Fatalf("update body = %q", input["body"])
			}
			_, _ = io.WriteString(w, `{"id":"11111111-1111-1111-1111-111111111111","title":"Draft","body":"`+strings.ReplaceAll(input["body"], "\n", "\\n")+`","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:01:00Z"}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	newOut := runCommand(t, []string{"new", "Draft"}, dir, server.Client())
	if !strings.Contains(newOut, "Created 11111111-1111-1111-1111-111111111111") {
		t.Fatalf("new output = %s", newOut)
	}
	listOut := runCommand(t, []string{"list"}, dir, server.Client())
	if !strings.Contains(listOut, "Draft") {
		t.Fatalf("list output = %s", listOut)
	}
	catOut := runCommand(t, []string{"cat", "11111111-1111-1111-1111-111111111111"}, dir, server.Client())
	if catOut != "# Draft" {
		t.Fatalf("cat output = %q", catOut)
	}
	pullOut := runCommand(t, []string{"pull", "11111111-1111-1111-1111-111111111111"}, dir, server.Client())
	if pullOut != "# Draft" {
		t.Fatalf("pull output = %q", pullOut)
	}
	file := filepath.Join(t.TempDir(), "append.md")
	if err := os.WriteFile(file, []byte("More\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	appendOut := runCommand(t, []string{"append", "11111111-1111-1111-1111-111111111111", file}, dir, server.Client())
	if !strings.Contains(appendOut, "Updated 11111111-1111-1111-1111-111111111111") {
		t.Fatalf("append output = %s", appendOut)
	}
	pushFile := filepath.Join(t.TempDir(), "push.md")
	if err := os.WriteFile(pushFile, []byte("Pushed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pushOut := runCommand(t, []string{"push", "11111111-1111-1111-1111-111111111111", pushFile}, dir, server.Client())
	if !strings.Contains(pushOut, "Updated 11111111-1111-1111-1111-111111111111") {
		t.Fatalf("push output = %s", pushOut)
	}
	replaceFile := filepath.Join(t.TempDir(), "replace.md")
	if err := os.WriteFile(replaceFile, []byte("Replaced\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replaceOut := runCommand(t, []string{"replace", "11111111-1111-1111-1111-111111111111", replaceFile}, dir, server.Client())
	if !strings.Contains(replaceOut, "Updated 11111111-1111-1111-1111-111111111111") {
		t.Fatalf("replace output = %s", replaceOut)
	}
	if len(requests) == 0 {
		t.Fatal("no requests recorded")
	}
}

func TestRunDocumentCommandsJSON(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"documents":[{"id":"doc-1","title":"One","body":"# One\n","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	out := runCommand(t, []string{"list", "--json"}, dir, server.Client())
	var parsed struct {
		Documents []struct {
			ID string `json:"id"`
		} `json:"documents"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid json %q: %v", out, err)
	}
	if len(parsed.Documents) != 1 || parsed.Documents[0].ID != "doc-1" {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestRunDocumentCommandsMissingAuthFails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunWithRuntime([]string{"list"}, Runtime{
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

func TestRunDocumentCommandsReportAPIErrors(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"document not found"}`)
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunWithRuntime([]string{"cat", "missing"}, Runtime{
		Stdout:    &stdout,
		Stderr:    &stderr,
		ConfigDir: dir,
		Env:       map[string]string{},
		HTTP:      server.Client(),
		Build:     BuildInfo{Version: "test"},
	})
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "document not found") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func runCommand(t *testing.T, args []string, dir string, client *http.Client) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunWithRuntime(args, Runtime{
		Stdout:    &stdout,
		Stderr:    &stderr,
		ConfigDir: dir,
		Env:       map[string]string{},
		HTTP:      client,
		Build:     BuildInfo{Version: "test"},
	})
	if code != 0 {
		t.Fatalf("%v code = %d, stderr = %s", args, code, stderr.String())
	}
	return stdout.String()
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
