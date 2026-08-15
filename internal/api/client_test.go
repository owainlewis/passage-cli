package api

import (
	"encoding/json"
	"io"
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

func TestClientDeletesDocument(t *testing.T) {
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

	client := Client{BaseURL: server.URL, Token: "psg_test", HTTP: server.Client()}
	if err := client.Delete("doc-1"); err != nil {
		t.Fatal(err)
	}
}

func TestClientRequiresToken(t *testing.T) {
	_, err := Client{BaseURL: "http://example.test"}.List()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("err = %v", err)
	}
}

func TestClientFollowsCollectionDocumentAndSearchPagination(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/collections":
			if r.URL.Query().Get("cursor") == "collection-next" {
				_, _ = io.WriteString(w, `{"collections":[{"id":"collection-2","slug":"research","title":"Research","description":null,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"collections":[{"id":"collection-1","slug":"operating-context","title":"Operating Context","description":null,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}],"nextCursor":"collection-next"}`)
		case "/api/v1/docs":
			if r.URL.Query().Get("limit") != "100" {
				t.Fatalf("list limit = %q", r.URL.Query().Get("limit"))
			}
			if r.URL.Query().Get("cursor") == "document-next" {
				_, _ = io.WriteString(w, `{"documents":[{"id":"doc-2","publicId":"public-2","title":"Two","excerpt":"Two","tags":[],"collectionId":null,"collectionSlug":null,"starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"documents":[{"id":"doc-1","publicId":"public-1","title":"One","excerpt":"One","tags":[],"collectionId":"collection-1","collectionSlug":"operating-context","starred":true,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}],"nextCursor":"document-next"}`)
		case "/api/v1/docs/search":
			if r.URL.Query().Get("q") != "agent workflow" || r.URL.Query().Get("collectionId") != "collection-1" || r.URL.Query().Get("limit") != "100" {
				t.Fatalf("search query = %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("cursor") == "search-next" {
				_, _ = io.WriteString(w, `{"documents":[{"id":"doc-2","publicId":"public-2","title":"Two","matchExcerpt":"agent workflow","tags":[],"collectionId":"collection-1","collectionSlug":"operating-context","starred":false,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"documents":[{"id":"doc-1","publicId":"public-1","title":"One","matchExcerpt":"agent workflow","tags":[],"collectionId":"collection-1","collectionSlug":"operating-context","starred":true,"createdAt":"2026-06-28T12:00:00Z","updatedAt":"2026-06-28T12:00:00Z"}],"nextCursor":"search-next"}`)
		default:
			t.Fatalf("request = %s", r.URL.RequestURI())
		}
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, Token: "psg_test", HTTP: server.Client()}
	collections, err := client.ListCollections()
	if err != nil || len(collections) != 2 {
		t.Fatalf("collections = %#v, err = %v", collections, err)
	}
	documents, err := client.ListMetadata()
	if err != nil || len(documents) != 2 {
		t.Fatalf("documents = %#v, err = %v", documents, err)
	}
	collectionID := "collection-1"
	results, err := client.Search("agent workflow", &collectionID, false, 0)
	if err != nil || len(results) != 2 {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
	if len(requests) != 6 {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestClientSearchLimitCapsResultsAndPageSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/docs/search" || r.URL.Query().Get("limit") != "2" || r.URL.Query().Get("unfiled") != "true" {
			t.Fatalf("request = %s", r.URL.RequestURI())
		}
		_, _ = io.WriteString(w, `{"documents":[{"id":"doc-1"},{"id":"doc-2"}],"nextCursor":"unused"}`)
	}))
	defer server.Close()

	results, err := (Client{BaseURL: server.URL, Token: "psg_test", HTTP: server.Client()}).Search("agent", nil, true, 2)
	if err != nil || len(results) != 2 {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
}
