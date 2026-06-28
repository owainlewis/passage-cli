package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientSendsBearerTokenAndCreatesDocument(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/docs" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var input map[string]string
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input["body"] != "# One\n" {
			t.Fatalf("body = %q", input["body"])
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"doc-1","title":"One","body":"# One\n","createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}`))
	}))
	defer server.Close()

	doc, err := Client{BaseURL: server.URL, Token: "psg_test", HTTP: server.Client()}.Create("# One\n")
	if err != nil {
		t.Fatal(err)
	}
	if authHeader != "Bearer psg_test" {
		t.Fatalf("authorization = %q", authHeader)
	}
	if doc.ID != "doc-1" || doc.Title != "One" {
		t.Fatalf("doc = %#v", doc)
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"authentication required"}`))
	}))
	defer server.Close()

	_, err := Client{BaseURL: server.URL, Token: "psg_bad", HTTP: server.Client()}.List()
	if err == nil || !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("err = %v", err)
	}
}

func TestClientSharesAndUnsharesDocument(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer psg_test" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docs/doc-1/share":
			_, _ = w.Write([]byte(`{"token":"sharetoken","htmlPath":"/d/sharetoken","markdownPath":"/d/sharetoken.md"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/docs/doc-1/share":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, Token: "psg_test", HTTP: server.Client()}
	share, err := client.Share("doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if share.HTMLPath != "/d/sharetoken" || share.MarkdownPath != "/d/sharetoken.md" {
		t.Fatalf("share = %#v", share)
	}
	if err := client.Unshare("doc-1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /api/v1/docs/doc-1/share", "DELETE /api/v1/docs/doc-1/share"}
	if strings.Join(requests, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestClientRequiresToken(t *testing.T) {
	_, err := Client{BaseURL: "http://example.test"}.List()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("err = %v", err)
	}
}
