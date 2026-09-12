package infrai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestSendVerificationRequestBoundary(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", req.Method)
		}
		if req.URL.Path != "/v1/email/send" {
			t.Fatalf("path = %q, want /v1/email/send", req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header was not set")
		}
		if req.Header.Get("Idempotency-Key") != "signup-7" {
			t.Fatalf("idempotency key = %q, want signup-7", req.Header.Get("Idempotency-Key"))
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["body"] != "Open this link to verify your email and release processed media: https://media.example/verify?token=7" {
			t.Fatalf("body = %q, want verification message", body["body"])
		}
		if _, exists := body["text"]; exists {
			t.Fatal("request contains unsupported text field")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"data":{"message_id":"message-7"},"metadata":{}}`)),
		}, nil
	})
	client := New("test-key", &http.Client{Transport: transport})

	messageID, err := client.SendVerification(context.Background(), "creator@example.com", "https://media.example/verify?token=7", "signup-7")
	if err != nil {
		t.Fatal(err)
	}
	if messageID != "message-7" {
		t.Fatalf("message ID = %q, want message-7", messageID)
	}
}
