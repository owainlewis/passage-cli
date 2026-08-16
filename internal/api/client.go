package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type Document struct {
	ID             string     `json:"id"`
	PublicID       string     `json:"publicId"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	CollectionID   *string    `json:"collectionId"`
	CollectionSlug *string    `json:"collectionSlug"`
	Starred        bool       `json:"starred"`
	Version        int        `json:"version"`
	ShareToken     *string    `json:"shareToken,omitempty"`
	SharedAt       *time.Time `json:"sharedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	ArchivedAt     *time.Time `json:"archivedAt,omitempty"`
}

// ConflictError reports that the document moved on between the read and the
// write. The caller must not retry with a forced overwrite: that is the
// silent data loss this whole mechanism exists to prevent.
type ConflictError struct {
	Current Document
}

func (e *ConflictError) Error() string {
	return "document changed since it was read"
}

type DocumentMetadata struct {
	ID             string     `json:"id"`
	PublicID       string     `json:"publicId"`
	Title          string     `json:"title"`
	Excerpt        string     `json:"excerpt"`
	Tags           []string   `json:"tags"`
	CollectionID   *string    `json:"collectionId"`
	CollectionSlug *string    `json:"collectionSlug"`
	Starred        bool       `json:"starred"`
	ShareToken     *string    `json:"shareToken,omitempty"`
	SharedAt       *time.Time `json:"sharedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type SearchResult struct {
	ID             string     `json:"id"`
	PublicID       string     `json:"publicId"`
	Title          string     `json:"title"`
	MatchExcerpt   string     `json:"matchExcerpt"`
	Tags           []string   `json:"tags"`
	CollectionID   *string    `json:"collectionId"`
	CollectionSlug *string    `json:"collectionSlug"`
	Starred        bool       `json:"starred"`
	ShareToken     *string    `json:"shareToken,omitempty"`
	SharedAt       *time.Time `json:"sharedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Collection struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Share struct {
	Token        string `json:"token"`
	HTMLPath     string `json:"htmlPath"`
	MarkdownPath string `json:"markdownPath"`
}

func (c Client) List() ([]Document, error) {
	var out struct {
		Documents []Document `json:"documents"`
	}
	if err := c.do(http.MethodGet, "/api/v1/docs", nil, &out); err != nil {
		return nil, err
	}
	if out.Documents == nil {
		out.Documents = []Document{}
	}
	return out.Documents, nil
}

func (c Client) ListMetadata() ([]DocumentMetadata, error) {
	var documents []DocumentMetadata
	cursor := ""
	seen := map[string]bool{}
	for {
		query := url.Values{"limit": {"100"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var page struct {
			Documents  []DocumentMetadata `json:"documents"`
			NextCursor string             `json:"nextCursor"`
		}
		if err := c.do(http.MethodGet, "/api/v1/docs?"+query.Encode(), nil, &page); err != nil {
			return nil, err
		}
		documents = append(documents, page.Documents...)
		if page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return nil, errors.New("server repeated a document cursor")
		}
		seen[page.NextCursor] = true
		cursor = page.NextCursor
	}
	if documents == nil {
		documents = []DocumentMetadata{}
	}
	return documents, nil
}

func (c Client) ListCollections() ([]Collection, error) {
	var collections []Collection
	path := "/api/v1/collections"
	seen := map[string]bool{}
	for {
		var page struct {
			Collections []Collection `json:"collections"`
			NextCursor  string       `json:"nextCursor"`
		}
		if err := c.do(http.MethodGet, path, nil, &page); err != nil {
			return nil, err
		}
		collections = append(collections, page.Collections...)
		if page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return nil, errors.New("server repeated a collection cursor")
		}
		seen[page.NextCursor] = true
		path = "/api/v1/collections?" + url.Values{"cursor": {page.NextCursor}}.Encode()
	}
	if collections == nil {
		collections = []Collection{}
	}
	return collections, nil
}

func (c Client) CreateCollection(title string, description *string) (Collection, error) {
	var collection Collection
	err := c.do(http.MethodPost, "/api/v1/collections", map[string]any{
		"title": title, "description": description,
	}, &collection)
	return collection, err
}

func (c Client) UpdateCollection(slug string, title string, description *string) (Collection, error) {
	var collection Collection
	err := c.do(http.MethodPatch, "/api/v1/collections/"+url.PathEscape(slug), map[string]any{
		"title": title, "description": description,
	}, &collection)
	return collection, err
}

func (c Client) DeleteCollection(slug string) error {
	return c.do(http.MethodDelete, "/api/v1/collections/"+url.PathEscape(slug), nil, nil)
}

func (c Client) Move(id string, collectionID *string) (Document, error) {
	var doc Document
	err := c.do(http.MethodPatch, "/api/v1/docs/"+url.PathEscape(id), map[string]any{
		"collectionId": collectionID,
	}, &doc)
	return doc, err
}

func (c Client) SetStarred(id string, starred bool) (Document, error) {
	var doc Document
	err := c.do(http.MethodPatch, "/api/v1/docs/"+url.PathEscape(id), map[string]any{
		"starred": starred,
	}, &doc)
	return doc, err
}

func (c Client) Search(query string, collectionID *string, unfiled bool, limit int) ([]SearchResult, error) {
	var results []SearchResult
	cursor := ""
	seen := map[string]bool{}
	for limit <= 0 || len(results) < limit {
		pageSize := 100
		if limit > 0 && limit-len(results) < pageSize {
			pageSize = limit - len(results)
		}
		values := url.Values{
			"q":     {query},
			"limit": {fmt.Sprintf("%d", pageSize)},
		}
		if collectionID != nil {
			values.Set("collectionId", *collectionID)
		}
		if unfiled {
			values.Set("unfiled", "true")
		}
		if cursor != "" {
			values.Set("cursor", cursor)
		}
		var page struct {
			Documents  []SearchResult `json:"documents"`
			NextCursor string         `json:"nextCursor"`
		}
		if err := c.do(http.MethodGet, "/api/v1/docs/search?"+values.Encode(), nil, &page); err != nil {
			return nil, err
		}
		results = append(results, page.Documents...)
		if page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return nil, errors.New("server repeated a search cursor")
		}
		seen[page.NextCursor] = true
		cursor = page.NextCursor
	}
	if results == nil {
		results = []SearchResult{}
	}
	return results, nil
}

func (c Client) Create(body string) (Document, error) {
	var doc Document
	err := c.do(http.MethodPost, "/api/v1/docs", map[string]string{"body": body}, &doc)
	return doc, err
}

func (c Client) Get(id string) (Document, error) {
	var doc Document
	err := c.do(http.MethodGet, "/api/v1/docs/"+url.PathEscape(id), nil, &doc)
	return doc, err
}

// Update writes unconditionally. Prefer UpdateAtVersion for anything that read
// the document first.
func (c Client) Update(id string, body string) (Document, error) {
	var doc Document
	err := c.do(http.MethodPatch, "/api/v1/docs/"+url.PathEscape(id), map[string]string{"body": body}, &doc)
	return doc, err
}

// UpdateAtVersion writes only if the document is still at version. A mismatch
// returns *ConflictError carrying the server's current copy.
func (c Client) UpdateAtVersion(id string, body string, version int) (Document, error) {
	var doc Document
	err := c.do(http.MethodPatch, "/api/v1/docs/"+url.PathEscape(id), map[string]any{"body": body, "version": version}, &doc)
	return doc, err
}

func (c Client) Delete(id string) error {
	return c.do(http.MethodDelete, "/api/v1/docs/"+url.PathEscape(id), nil, nil)
}

func (c Client) Share(id string) (Share, error) {
	var share Share
	err := c.do(http.MethodPost, "/api/v1/docs/"+url.PathEscape(id)+"/share", nil, &share)
	return share, err
}

func (c Client) Unshare(id string) error {
	return c.do(http.MethodDelete, "/api/v1/docs/"+url.PathEscape(id)+"/share", nil, nil)
}

func (c Client) do(method string, path string, input any, output any) error {
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("not authenticated")
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusConflict {
		var conflict struct {
			Error    string   `json:"error"`
			Document Document `json:"document"`
		}
		if json.Unmarshal(data, &conflict) == nil && conflict.Document.ID != "" {
			return &ConflictError{Current: conflict.Document}
		}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("%s", apiErr.Error)
		}
		return fmt.Errorf("server returned %d", res.StatusCode)
	}
	if output == nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, output)
}
