package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	var auth AuthResponse
	if err := json.NewDecoder(res.Body).Decode(&auth); err != nil {
		return AuthResponse{}, res.StatusCode, fmt.Errorf("decode auth response: %w", err)
	}

	if res.StatusCode >= 500 {
		return auth, res.StatusCode, errors.New("api internal auth failed")
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

	var payload PostReceiveResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return PostReceiveResponse{}, res.StatusCode, fmt.Errorf("decode post-receive response: %w", err)
	}

	if res.StatusCode >= 400 {
		return payload, res.StatusCode, errors.New("api post-receive failed")
	}

	return payload, res.StatusCode, nil
}
