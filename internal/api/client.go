package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const internalSecretHeader = "x-gitta-internal-secret"

type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

func NewClient(baseURL, secret string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) Secret() string {
	return c.secret
}

func (c *Client) Authorize(ctx context.Context, input AuthRequest) (AuthResponse, int, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return AuthResponse{}, 0, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/git/auth",
		bytes.NewReader(body),
	)
	if err != nil {
		return AuthResponse{}, 0, err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set(internalSecretHeader, c.secret)

	res, err := c.http.Do(req)
	if err != nil {
		return AuthResponse{}, 0, err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return AuthResponse{}, res.StatusCode, fmt.Errorf("read auth response: %w", err)
	}

	var auth AuthResponse
	if err := json.Unmarshal(responseBody, &auth); err != nil {
		return AuthResponse{}, res.StatusCode, fmt.Errorf("decode auth response status=%d body=%q: %w", res.StatusCode, string(responseBody), err)
	}

	if res.StatusCode >= 500 {
		return auth, res.StatusCode, fmt.Errorf("api internal auth failed status=%d reason=%q", res.StatusCode, auth.Reason)
	}

	return auth, res.StatusCode, nil
}

func (c *Client) PostReceive(ctx context.Context, input PostReceiveRequest) (PostReceiveResponse, int, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return PostReceiveResponse{}, 0, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/git/post-receive",
		bytes.NewReader(body),
	)
	if err != nil {
		return PostReceiveResponse{}, 0, err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set(internalSecretHeader, c.secret)

	res, err := c.http.Do(req)
	if err != nil {
		return PostReceiveResponse{}, 0, err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return PostReceiveResponse{}, res.StatusCode, fmt.Errorf("read post-receive response: %w", err)
	}

	var payload PostReceiveResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return PostReceiveResponse{}, res.StatusCode, fmt.Errorf("decode post-receive response status=%d body=%q: %w", res.StatusCode, string(responseBody), err)
	}

	if res.StatusCode >= 400 {
		return payload, res.StatusCode, fmt.Errorf("api post-receive failed status=%d reason=%q", res.StatusCode, payload.Reason)
	}

	return payload, res.StatusCode, nil
}

func (c *Client) PreReceive(ctx context.Context, input PreReceiveRequest) (PreReceiveResponse, int, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return PreReceiveResponse{}, 0, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/git/pre-receive",
		bytes.NewReader(body),
	)
	if err != nil {
		return PreReceiveResponse{}, 0, err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set(internalSecretHeader, c.secret)

	res, err := c.http.Do(req)
	if err != nil {
		return PreReceiveResponse{}, 0, err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return PreReceiveResponse{}, res.StatusCode, fmt.Errorf("read pre-receive response: %w", err)
	}

	var payload PreReceiveResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return PreReceiveResponse{}, res.StatusCode, fmt.Errorf("decode pre-receive response status=%d body=%q: %w", res.StatusCode, string(responseBody), err)
	}

	if res.StatusCode >= 400 {
		return payload, res.StatusCode, fmt.Errorf("api pre-receive failed status=%d reason=%q", res.StatusCode, payload.Reason)
	}

	return payload, res.StatusCode, nil
}
