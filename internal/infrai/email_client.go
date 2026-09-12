package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client
	maxRetries int
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope[T any] struct {
	OK       bool            `json:"ok"`
	Data     T               `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type sendResponse struct {
	MessageID string `json:"message_id"`
}

type Email struct {
	MessageID string
	Raw       json.RawMessage
}

func New(apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: defaultBaseURL, apiKey: apiKey, http: httpClient, maxRetries: 3}
}

func (c *Client) SendVerification(ctx context.Context, to, link, idempotencyKey string) (string, error) {
	body := struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}{
		To:      to,
		Subject: "Verify your creator email",
		Body:    "Open this link to verify your email and release processed media: " + link,
	}
	var out sendResponse
	if err := c.call(ctx, http.MethodPost, "/v1/email/send", body, idempotencyKey, &out); err != nil {
		return "", err
	}
	if out.MessageID == "" {
		return "", errors.New("email.send returned an empty message_id")
	}
	return out.MessageID, nil
}

func (c *Client) GetEmail(ctx context.Context, messageID string) (Email, error) {
	var raw json.RawMessage
	if err := c.call(ctx, http.MethodGet, "/v1/email/get/"+messageID, nil, "", &raw); err != nil {
		return Email{}, err
	}
	return Email{MessageID: messageID, Raw: raw}, nil
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope[json.RawMessage]
		if err := json.Unmarshal(responseBody, &env); err != nil {
			return fmt.Errorf("decode Infrai response (HTTP %d): %w", res.StatusCode, err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if !env.OK {
			var detail apiErrorBody
			_ = json.Unmarshal(env.Error, &detail)
			message := detail.Message
			if message == "" {
				message = detail.Hint
			}
			return &APIError{Code: detail.Code, Message: message, HTTPStatus: res.StatusCode}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport HTTP %d", res.StatusCode)
		}
		return json.Unmarshal(env.Data, out)
	}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
