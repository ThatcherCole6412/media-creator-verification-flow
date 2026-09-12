package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"example.com/media-verification-flow/internal/flow"
	"example.com/media-verification-flow/internal/infrai"
)

type mailerAdapter struct{ client *infrai.Client }

func (m mailerAdapter) SendVerification(ctx context.Context, to, link, key string) (string, error) {
	return m.client.SendVerification(ctx, to, link, key)
}

func (m mailerAdapter) GetEmail(ctx context.Context, id string) (any, error) {
	return m.client.GetEmail(ctx, id)
}

type server struct{ flow *flow.Service }

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	publicURL := envOr("PUBLIC_URL", "http://127.0.0.1:8080")
	s := &server{flow: flow.NewService(mailerAdapter{client: infrai.New(key, nil)}, publicURL)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", s.signup)
	mux.HandleFunc("GET /verify", s.verify)
	addr := envOr("LISTEN_ADDR", ":8080")
	log.Printf("media signup service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email  string `json:"email"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	result, err := s.flow.Signup(r.Context(), input.Email, input.Source)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *server) verify(w http.ResponseWriter, r *http.Request) {
	result, err := s.flow.Verify(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Error()})
		return
	}
	if err.Error() == "invalid verification token" || err.Error() == "email and source are required" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "email delivery request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
