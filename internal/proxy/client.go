package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Sn0wo2/grok-cli2api/internal/auth"
	"github.com/Sn0wo2/grok-cli2api/internal/config"
)

type Client struct {
	base   string
	http   *http.Client
	stream *http.Client
}

func NewClient(log *slog.Logger) *Client {
	return &Client{
		base:   config.ProxyBaseURL(),
		http:   &http.Client{Timeout: 30 * time.Second},
		stream: &http.Client{},
	}
}

func (c *Client) GetModels(rec auth.AccountRecord) (*http.Response, error) {
	return c.do(http.MethodGet, "/models", rec, nil, "")
}

func (c *Client) PostResponses(rec auth.AccountRecord, body []byte, model string) (*http.Response, error) {
	return c.do(http.MethodPost, "/responses", rec, bytes.NewReader(body), model)
}

func (c *Client) do(method, path string, rec auth.AccountRecord, body io.Reader, model string) (*http.Response, error) {
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	auth.SetGrokHeaders(req, rec, model)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := c.http
	if method == http.MethodPost && path == "/responses" {
		client = c.stream
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy request: %w", err)
	}
	return resp, nil
}

func ExtractModel(body []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &payload)
	return payload.Model
}
