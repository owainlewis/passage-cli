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

func TestRunSubcommandShowsHelpWithoutRequest(t *testing.T) {
	for _, command := range []string{"new", "delete", "collection", "move", "star", "unstar", "search"} {
		for _, helpFlag := range []string{"-h", "--help"} {
			t.Run(command+"/"+helpFlag, func(t *testing.T) {
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					w.WriteHeader(http.StatusInternalServerError)
				}))
				defer server.Close()

				var stdout bytes.Buffer
				var stderr bytes.Buffer
				code := RunWithRuntime([]string{command, helpFlag}, Runtime{
					Stdout:    &stdout,
					Stderr:    &stderr,
					ConfigDir: t.TempDir(),
					Env: map[string]string{
						"PASSAGE_API_URL": server.URL,
						"PASSAGE_TOKEN":   "psg_testtoken",
					},
					HTTP:  server.Client(),
					Build: BuildInfo{Version: "test"},
				})

				if code != 0 {
					t.Fatalf("code = %d, stderr = %s", code, stderr.String())
				}
				if !strings.Contains(stdout.String(), "Usage:") {
					t.Fatalf("stdout = %q", stdout.String())
				}
				if stderr.Len() != 0 {
					t.Fatalf("stderr = %q", stderr.String())
				}
				if requests != 0 {
					t.Fatalf("requests = %d, want 0", requests)
				}
			})
		}
	}
}

func TestRunUnknownCommandWithHelpReturnsError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := RunWithRuntime([]string{"nwe", "--help"}, Runtime{
		Stdout: &stdout,
		Stderr: &stderr,
		Build:  BuildInfo{Version: "test"},
	})

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "nwe"`) {
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

func TestRunCollectionCommandsPlainAndJSON(t *testing.T) {
	dir := t.TempDir()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer psg_test" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/collections":
			_, _ = io.WriteString(w, `{"collections":[{"id":"collection-default","slug":"operating-context","title":"Operating Context","description":"Goals and rules","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"},{"id":"collection-custom","slug":"custom","title":"Custom","description":"Old","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/collections":
			var input struct {
				Title       string  `json:"title"`
				Description *string `json:"description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input.Title != "Customer Research" || input.Description == nil || *input.Description != "Interviews" {
				t.Fatalf("create input = %#v", input)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":"collection-new","slug":"customer-research","title":"Customer Research","description":"Interviews","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/collections/custom":
			var input struct {
				Title       string  `json:"title"`
				Description *string `json:"description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input.Title != "Renamed" || input.Description == nil || *input.Description != "Old" {
				t.Fatalf("update input = %#v", input)
			}
			_, _ = io.WriteString(w, `{"id":"collection-custom","slug":"custom","title":"Renamed","description":"Old","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:01:00Z"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/collections/custom":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("request = %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	plain := runCommand(t, []string{"collection", "list"}, dir, server.Client())
	if plain != "operating-context\tOperating Context\tGoals and rules\ncustom\tCustom\tOld\n" {
		t.Fatalf("list output = %q", plain)
	}
	listJSON := runCommand(t, []string{"collection", "list", "--json"}, dir, server.Client())
	var listed struct {
		Collections []struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"collections"`
	}
	if err := json.Unmarshal([]byte(listJSON), &listed); err != nil || len(listed.Collections) != 2 || listed.Collections[0].ID == "" {
		t.Fatalf("collection JSON = %q, parsed = %#v, err = %v", listJSON, listed, err)
	}
	createdJSON := runCommand(t, []string{"collection", "create", "Customer Research", "--description", "Interviews", "--json"}, dir, server.Client())
	var created struct {
		ID        string `json:"id"`
		Slug      string `json:"slug"`
		CreatedAt string `json:"createdAt"`
	}
	if err := json.Unmarshal([]byte(createdJSON), &created); err != nil || created.ID != "collection-new" || created.Slug != "customer-research" || created.CreatedAt == "" {
		t.Fatalf("create JSON = %q, parsed = %#v, err = %v", createdJSON, created, err)
	}
	updated := runCommand(t, []string{"collection", "update", "custom", "--title", "Renamed"}, dir, server.Client())
	if strings.TrimSpace(updated) != "Updated custom\tRenamed" {
		t.Fatalf("update output = %q", updated)
	}

	beforeCancel := requests
	var cancelOut bytes.Buffer
	var cancelErr bytes.Buffer
	code := RunWithRuntime([]string{"collection", "delete", "custom"}, Runtime{
		Stdin: strings.NewReader("n\n"), Stdout: &cancelOut, Stderr: &cancelErr,
		ConfigDir: dir, Env: map[string]string{}, HTTP: server.Client(),
	})
	if code != 0 || cancelOut.Len() != 0 || !strings.Contains(cancelErr.String(), "Deletion cancelled") || requests != beforeCancel {
		t.Fatalf("cancel code/out/err/requests = %d/%q/%q/%d", code, cancelOut.String(), cancelErr.String(), requests)
	}
	deleted := runCommand(t, []string{"collection", "delete", "custom", "--yes"}, dir, server.Client())
	if strings.TrimSpace(deleted) != "Deleted collection custom" {
		t.Fatalf("delete output = %q", deleted)
	}
}

func TestRunScopedListMoveStarAndSearch(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/collections":
			_, _ = io.WriteString(w, `{"collections":[{"id":"collection-1","slug":"operating-context","title":"Operating Context","description":null,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docs":
			if r.URL.Query().Get("limit") != "100" {
				t.Fatalf("list query = %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"documents":[{"id":"doc-collected","publicId":"public-collected","title":"Collected\nTitle","excerpt":"# Collected","tags":["agents"],"collectionId":"collection-1","collectionSlug":"operating-context","starred":true,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:01:00Z"},{"id":"doc-unfiled","publicId":"public-unfiled","title":"Unfiled","excerpt":"# Unfiled","tags":[],"collectionId":null,"collectionSlug":null,"starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/docs/"):
			var input map[string]any
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			collectionID := any(nil)
			collectionSlug := any(nil)
			starred := false
			if value, ok := input["collectionId"]; ok && value != nil {
				collectionID = value
				collectionSlug = "operating-context"
			}
			if value, ok := input["starred"].(bool); ok {
				starred = value
			}
			response := map[string]any{
				"id": strings.TrimPrefix(r.URL.Path, "/api/v1/docs/"), "publicId": "public-1", "title": "Agent\tPlan", "body": "# Agent Plan\n",
				"collectionId": collectionID, "collectionSlug": collectionSlug, "starred": starred,
				"createdAt": "2026-06-28T12:00:00Z", "updatedAt": "2026-06-28T12:01:00Z",
			}
			_ = json.NewEncoder(w).Encode(response)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docs/search":
			if r.URL.Query().Get("q") != "agent workflow" {
				t.Fatalf("search query = %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("collectionId") != "collection-1" && r.URL.Query().Get("unfiled") != "true" {
				t.Fatalf("search scope = %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"documents":[{"id":"doc-search","publicId":"public-search","title":"Agent\nWorkflow","matchExcerpt":"after\nthe first 4 KB","tags":["agents"],"collectionId":"collection-1","collectionSlug":"operating-context","starred":true,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:01:00Z"}]}`)
		default:
			t.Fatalf("request = %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	collected := runCommand(t, []string{"list", "--collection", "operating-context"}, dir, server.Client())
	if collected != "doc-collected\t2026-06-28 12:01\tCollected Title\n" || strings.Contains(collected, "doc-unfiled") {
		t.Fatalf("collected output = %q", collected)
	}
	unfiledJSON := runCommand(t, []string{"list", "--collection", "documents", "--json"}, dir, server.Client())
	if !strings.Contains(unfiledJSON, `"id": "doc-unfiled"`) || strings.Contains(unfiledJSON, `"id": "doc-collected"`) || !strings.Contains(unfiledJSON, `"collectionId": null`) {
		t.Fatalf("unfiled JSON = %q", unfiledJSON)
	}
	movedJSON := runCommand(t, []string{"move", "doc-1", "--collection", "operating-context", "--json"}, dir, server.Client())
	for _, field := range []string{`"id": "doc-1"`, `"publicId": "public-1"`, `"collectionSlug": "operating-context"`, `"starred": false`, `"updatedAt":`} {
		if !strings.Contains(movedJSON, field) {
			t.Fatalf("move JSON %q missing %q", movedJSON, field)
		}
	}
	movedDocuments := runCommand(t, []string{"move", "doc-1", "--collection", "documents"}, dir, server.Client())
	if strings.TrimSpace(movedDocuments) != "Moved doc-1\tdocuments" {
		t.Fatalf("move Documents output = %q", movedDocuments)
	}
	starred := runCommand(t, []string{"star", "doc-1"}, dir, server.Client())
	if strings.TrimSpace(starred) != "Starred doc-1\tAgent Plan" {
		t.Fatalf("star output = %q", starred)
	}
	unstarredJSON := runCommand(t, []string{"unstar", "doc-1", "--json"}, dir, server.Client())
	if !strings.Contains(unstarredJSON, `"starred": false`) {
		t.Fatalf("unstar JSON = %q", unstarredJSON)
	}
	searchJSON := runCommand(t, []string{"search", "agent workflow", "--collection", "operating-context", "--limit", "1", "--json"}, dir, server.Client())
	for _, field := range []string{`"id": "doc-search"`, `"publicId": "public-search"`, `"collectionSlug": "operating-context"`, `"starred": true`, `"matchExcerpt":`} {
		if !strings.Contains(searchJSON, field) {
			t.Fatalf("search JSON %q missing %q", searchJSON, field)
		}
	}
	searchPlain := runCommand(t, []string{"search", "agent workflow", "--collection", "documents"}, dir, server.Client())
	if searchPlain != "doc-search\t2026-06-28 12:01\tAgent Workflow\tafter the first 4 KB\n" {
		t.Fatalf("search plain = %q", searchPlain)
	}
}

func TestRunNewCommandsFollowPagination(t *testing.T) {
	dir := t.TempDir()
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cursor := r.URL.Query().Get("cursor")
		cursors = append(cursors, r.URL.Path+":"+cursor)
		switch r.URL.Path {
		case "/api/v1/collections":
			if cursor == "collections-next" {
				_, _ = io.WriteString(w, `{"collections":[{"id":"collection-2","slug":"research","title":"Research","description":null,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
			} else {
				_, _ = io.WriteString(w, `{"collections":[{"id":"collection-1","slug":"operating-context","title":"Operating Context","description":null,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}],"nextCursor":"collections-next"}`)
			}
		case "/api/v1/docs":
			if cursor == "docs-next" {
				_, _ = io.WriteString(w, `{"documents":[{"id":"doc-2","publicId":"public-2","title":"Second","excerpt":"Second","tags":[],"collectionId":"collection-1","collectionSlug":"operating-context","starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
			} else {
				_, _ = io.WriteString(w, `{"documents":[{"id":"doc-1","publicId":"public-1","title":"First","excerpt":"First","tags":[],"collectionId":"collection-1","collectionSlug":"operating-context","starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:01:00Z"}],"nextCursor":"docs-next"}`)
			}
		case "/api/v1/docs/search":
			if cursor == "search-next" {
				_, _ = io.WriteString(w, `{"documents":[{"id":"doc-2","publicId":"public-2","title":"Second","matchExcerpt":"agent","tags":[],"collectionId":null,"collectionSlug":null,"starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
			} else {
				_, _ = io.WriteString(w, `{"documents":[{"id":"doc-1","publicId":"public-1","title":"First","matchExcerpt":"agent","tags":[],"collectionId":null,"collectionSlug":null,"starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:01:00Z"}],"nextCursor":"search-next"}`)
			}
		default:
			t.Fatalf("request = %s", r.URL.RequestURI())
		}
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	if out := runCommand(t, []string{"collection", "list"}, dir, server.Client()); !strings.Contains(out, "operating-context") || !strings.Contains(out, "research") {
		t.Fatalf("collection output = %q", out)
	}
	if out := runCommand(t, []string{"list", "--collection", "operating-context"}, dir, server.Client()); !strings.Contains(out, "doc-1") || !strings.Contains(out, "doc-2") {
		t.Fatalf("list output = %q", out)
	}
	if out := runCommand(t, []string{"search", "agent"}, dir, server.Client()); !strings.Contains(out, "doc-1") || !strings.Contains(out, "doc-2") {
		t.Fatalf("search output = %q", out)
	}
	for _, want := range []string{
		"/api/v1/collections:collections-next", "/api/v1/docs:docs-next", "/api/v1/docs/search:search-next",
	} {
		if !containsString(cursors, want) {
			t.Fatalf("cursors = %#v, missing %q", cursors, want)
		}
	}
}

func TestRunDeleteCommand(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer psg_test" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/docs/doc-1" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	out := runCommand(t, []string{"delete", "doc-1"}, dir, server.Client())
	if strings.TrimSpace(out) != "Deleted doc-1" {
		t.Fatalf("delete output = %s", out)
	}

	jsonOut := runCommand(t, []string{"delete", "--json", "doc-1"}, dir, server.Client())
	var parsed struct {
		Deleted bool   `json:"deleted"`
		DocID   string `json:"doc_id"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("invalid json %q: %v", jsonOut, err)
	}
	if !parsed.Deleted || parsed.DocID != "doc-1" {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestRunDeleteCommandReportsUsageAndAPIErrors(t *testing.T) {
	t.Run("missing document", func(t *testing.T) {
		var stderr bytes.Buffer
		code := RunWithRuntime([]string{"delete"}, Runtime{
			Stdout:    io.Discard,
			Stderr:    &stderr,
			ConfigDir: t.TempDir(),
			Env:       map[string]string{},
		})
		if code != 1 || !strings.Contains(stderr.String(), "usage: passage delete [--json] <doc>") {
			t.Fatalf("code = %d, stderr = %s", code, stderr.String())
		}
	})

	t.Run("shared document", func(t *testing.T) {
		dir := t.TempDir()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"error":"unshare this document before deleting it"}`)
		}))
		defer server.Close()
		if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
			t.Fatal(err)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := RunWithRuntime([]string{"delete", "doc-1"}, Runtime{
			Stdout:    &stdout,
			Stderr:    &stderr,
			ConfigDir: dir,
			Env:       map[string]string{},
			HTTP:      server.Client(),
		})
		if code != 1 || !strings.Contains(stderr.String(), "unshare this document before deleting it") {
			t.Fatalf("code = %d, stderr = %s", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %s", stdout.String())
		}
	})
}

func TestRunSharingCommands(t *testing.T) {
	dir := t.TempDir()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer psg_test" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docs/doc-1/share":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"token":"sharetoken","htmlPath":"/d/sharetoken","markdownPath":"/d/sharetoken.md"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/docs/doc-1/share":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	shareOut := runCommand(t, []string{"share", "doc-1"}, dir, server.Client())
	if !strings.Contains(shareOut, "Shared doc-1") {
		t.Fatalf("share output = %s", shareOut)
	}
	if !strings.Contains(shareOut, "HTML: "+server.URL+"/d/sharetoken") {
		t.Fatalf("share output = %s", shareOut)
	}
	if !strings.Contains(shareOut, "Raw: "+server.URL+"/d/sharetoken.md") {
		t.Fatalf("share output = %s", shareOut)
	}

	shareJSONOut := runCommand(t, []string{"share", "--json", "doc-1"}, dir, server.Client())
	assertShareJSON(t, shareJSONOut, server.URL)

	rawOut := runCommand(t, []string{"raw", "doc-1"}, dir, server.Client())
	if strings.TrimSpace(rawOut) != server.URL+"/d/sharetoken.md" {
		t.Fatalf("raw output = %s", rawOut)
	}

	jsonOut := runCommand(t, []string{"raw", "--json", "doc-1"}, dir, server.Client())
	assertShareJSON(t, jsonOut, server.URL)
	unshareOut := runCommand(t, []string{"unshare", "doc-1"}, dir, server.Client())
	if strings.TrimSpace(unshareOut) != "Unshared doc-1" {
		t.Fatalf("unshare output = %s", unshareOut)
	}
	if !strings.Contains(strings.Join(requests, "\n"), "DELETE /api/v1/docs/doc-1/share") {
		t.Fatalf("requests = %#v", requests)
	}
}

func assertShareJSON(t *testing.T, out string, baseURL string) {
	t.Helper()
	var parsed struct {
		DocID   string `json:"doc_id"`
		Token   string `json:"token"`
		HTMLURL string `json:"html_url"`
		RawURL  string `json:"raw_url"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid json %q: %v", out, err)
	}
	if parsed.DocID != "doc-1" || parsed.Token != "sharetoken" || parsed.HTMLURL != baseURL+"/d/sharetoken" || parsed.RawURL != baseURL+"/d/sharetoken.md" {
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

func TestRunNewCommandFailureClasses(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		status int
		body   string
		want   string
	}{
		{name: "validation", args: []string{"collection", "create", "Too long"}, status: http.StatusBadRequest, body: `{"error":"collection title is too long"}`, want: "collection title is too long"},
		{name: "auth", args: []string{"star", "doc-1"}, status: http.StatusUnauthorized, body: `{"error":"authentication required"}`, want: "authentication required"},
		{name: "not found", args: []string{"move", "missing", "--collection", "documents"}, status: http.StatusNotFound, body: `{"error":"document not found"}`, want: "document not found"},
		{name: "conflict limit", args: []string{"collection", "create", "One more"}, status: http.StatusConflict, body: `{"error":"collection limit reached"}`, want: "collection limit reached"},
		{name: "rate limit", args: []string{"search", "agent"}, status: http.StatusTooManyRequests, body: `{"error":"search rate limit exceeded"}`, want: "search rate limit exceeded"},
		{name: "server", args: []string{"collection", "list"}, status: http.StatusInternalServerError, body: `{"error":"collections could not be loaded"}`, want: "collections could not be loaded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer psg_secret_value" {
					t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := RunWithRuntime(test.args, Runtime{
				Stdout: &stdout, Stderr: &stderr, ConfigDir: t.TempDir(),
				Env: map[string]string{"PASSAGE_API_URL": server.URL, "PASSAGE_TOKEN": "psg_secret_value"}, HTTP: server.Client(),
			})
			if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("code/out/err = %d/%q/%q", code, stdout.String(), stderr.String())
			}
			if strings.Contains(stdout.String()+stderr.String(), "psg_secret_value") {
				t.Fatalf("output leaked token: %q %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunNewCommandsRejectInvalidArgumentsWithoutRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "collection missing subcommand", args: []string{"collection"}},
		{name: "collection create missing title", args: []string{"collection", "create"}},
		{name: "collection update missing change", args: []string{"collection", "update", "research"}},
		{name: "collection delete missing slug", args: []string{"collection", "delete"}},
		{name: "list missing collection value", args: []string{"list", "--collection"}},
		{name: "move missing collection", args: []string{"move", "doc-1"}},
		{name: "star missing document", args: []string{"star"}},
		{name: "unstar extra argument", args: []string{"unstar", "doc-1", "extra"}},
		{name: "search missing query", args: []string{"search"}},
		{name: "search invalid limit", args: []string{"search", "agent", "--limit", "0"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
			}))
			defer server.Close()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := RunWithRuntime(test.args, Runtime{
				Stdout: &stdout, Stderr: &stderr, ConfigDir: t.TempDir(),
				Env: map[string]string{"PASSAGE_API_URL": server.URL, "PASSAGE_TOKEN": "psg_test"}, HTTP: server.Client(),
			})
			if code == 0 || stdout.Len() != 0 || stderr.Len() == 0 || requests != 0 {
				t.Fatalf("code/out/err/requests = %d/%q/%q/%d", code, stdout.String(), stderr.String(), requests)
			}
		})
	}
}

func TestRunCollectionLookupReportsMissingSlug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"collections":[]}`)
	}))
	defer server.Close()
	for _, args := range [][]string{
		{"collection", "update", "missing", "--title", "New"},
		{"move", "doc-1", "--collection", "missing"},
		{"search", "agent", "--collection", "missing"},
		{"list", "--collection", "missing"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := RunWithRuntime(args, Runtime{
			Stdout: &stdout, Stderr: &stderr, ConfigDir: t.TempDir(),
			Env: map[string]string{"PASSAGE_API_URL": server.URL, "PASSAGE_TOKEN": "psg_test"}, HTTP: server.Client(),
		})
		if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `collection "missing" not found`) {
			t.Fatalf("%v code/out/err = %d/%q/%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunPushReplaceAndAppendReadStdinWithoutChangingMarkdown(t *testing.T) {
	dir := t.TempDir()
	var patches []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, `{"id":"doc-1","publicId":"public-1","title":"Existing","body":"Existing","collectionId":null,"collectionSlug":null,"starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}`)
		case http.MethodPatch:
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			patches = append(patches, input["body"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "doc-1", "publicId": "public-1", "title": "Updated", "body": input["body"],
				"collectionId": nil, "collectionSlug": nil, "starred": false,
				"createdAt": "2026-06-28T12:00:00Z", "updatedAt": "2026-06-28T12:01:00Z",
			})
		default:
			t.Fatalf("request = %s", r.Method)
		}
	}))
	defer server.Close()
	if err := config.Save(dir, config.Config{APIURL: server.URL, Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		command string
		stdin   string
	}{
		{command: "push", stdin: "# Pushed\n\nExact body.\n"},
		{command: "replace", stdin: "# Replaced\n\nExact body.\n"},
		{command: "append", stdin: "More\n"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := RunWithRuntime([]string{test.command, "doc-1", "-"}, Runtime{
			Stdin: strings.NewReader(test.stdin), Stdout: &stdout, Stderr: &stderr,
			ConfigDir: dir, Env: map[string]string{}, HTTP: server.Client(),
		})
		if code != 0 || stderr.Len() != 0 {
			t.Fatalf("%s code/err = %d/%q", test.command, code, stderr.String())
		}
	}
	want := []string{"# Pushed\n\nExact body.\n", "# Replaced\n\nExact body.\n", "Existing\nMore\n"}
	if strings.Join(patches, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("patches = %#v", patches)
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

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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
