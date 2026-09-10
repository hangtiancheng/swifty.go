// Package http provides a minimal JSON-over-HTTP client used by the
// timewheel's redis-backed time wheel to deliver task callbacks.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
)

// Client is a JSON HTTP client.
type Client struct {
	core *nethttp.Client
}

// NewClient creates a Client backed by http.DefaultClient.
func NewClient() *Client {
	return &Client{
		core: nethttp.DefaultClient,
	}
}

// JSONGet performs a GET request and decodes the JSON response into resp.
func (c *Client) JSONGet(ctx context.Context, url string, header, params map[string]string, resp interface{}) error {
	return c.JSONDo(ctx, nethttp.MethodGet, getCompleteURL(url, params), header, nil, resp)
}

// JSONPost performs a POST request with req serialized as the JSON body and
// decodes the JSON response into resp.
func (c *Client) JSONPost(ctx context.Context, url string, header map[string]string, req, resp interface{}) error {
	return c.JSONDo(ctx, nethttp.MethodPost, url, header, req, resp)
}

// JSONDo performs an HTTP request; req is serialized as a JSON body when not
// nil, and the JSON response is decoded into resp when not nil.
func (c *Client) JSONDo(ctx context.Context, method, url string, header map[string]string, req, resp interface{}) error {
	var reqReader io.Reader
	if req != nil {
		body, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqReader = bytes.NewReader(body)
	}

	request, err := nethttp.NewRequestWithContext(ctx, method, url, reqReader)
	if err != nil {
		return err
	}

	for k, v := range header {
		request.Header.Add(k, v)
	}
	if reqReader != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.core.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != nethttp.StatusOK {
		return fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	if resp == nil {
		return nil
	}

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	return json.Unmarshal(respBody, resp)
}

// getCompleteURL appends params to origin as a properly encoded query string.
func getCompleteURL(origin string, params map[string]string) string {
	if len(params) == 0 {
		return origin
	}
	values := url.Values{}
	for k, v := range params {
		values.Add(k, v)
	}
	return fmt.Sprintf("%s?%s", origin, values.Encode())
}
