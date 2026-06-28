package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type Document struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	ShareToken *string    `json:"shareToken,omitempty"`
	SharedAt   *time.Time `json:"sharedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	ArchivedAt *time.Time `json:"archivedAt,omitempty"`
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

func (c Client) Create(body string) (Document, error) {
	var doc Document
	err := c.do(http.MethodPost, "/api/v1/docs", map[string]string{"body": body}, &doc)
	return doc, err
}

func (c Client) Get(id string) (Document, error) {
	var doc Document
	err := c.do(http.MethodGet, "/api/v1/docs/"+id, nil, &doc)
	return doc, err
}

func (c Client) Update(id string, body string) (Document, error) {
	var doc Document
	err := c.do(http.MethodPatch, "/api/v1/docs/"+id, map[string]string{"body": body}, &doc)
	return doc, err
}

func (c Client) Share(id string) (Share, error) {
	var share Share
	err := c.do(http.MethodPost, "/api/v1/docs/"+id+"/share", nil, &share)
	return share, err
}

func (c Client) Unshare(id string) error {
	return c.do(http.MethodDelete, "/api/v1/docs/"+id+"/share", nil, nil)
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
