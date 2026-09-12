package flow

import (
	"context"
	"testing"
)

type fakeMailer struct {
	sentID  string
	checked string
}

func (f *fakeMailer) SendVerification(context.Context, string, string, string) (string, error) {
	return f.sentID, nil
}

func (f *fakeMailer) GetEmail(_ context.Context, id string) (any, error) {
	f.checked = id
	return struct{}{}, nil
}

func TestVerificationReleasesProcessedAsset(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantState string
		wantError bool
	}{
		{name: "matching token delivers asset", token: "token-1", wantState: "delivered"},
		{name: "unknown token keeps asset private", token: "wrong", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mailer := &fakeMailer{sentID: "message-42"}
			svc := NewService(mailer, "http://127.0.0.1:8080")
			svc.accounts["creator-1"] = &Account{ID: "creator-1", VerificationMessageID: "message-42"}
			svc.tokens["token-1"] = "creator-1"
			svc.assets["asset-1"] = &Asset{ID: "asset-1", CreatorID: "creator-1", State: "ingested"}
			svc.jobs["job-1"] = &ProcessingJob{ID: "job-1", AssetID: "asset-1", State: "queued"}

			got, err := svc.Verify(context.Background(), tt.token)
			if (err != nil) != tt.wantError {
				t.Fatalf("Verify() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError {
				if svc.assets["asset-1"].State != "ingested" {
					t.Fatalf("asset state = %q, want ingested", svc.assets["asset-1"].State)
				}
				return
			}
			if got.State != tt.wantState {
				t.Fatalf("delivery state = %q, want %q", got.State, tt.wantState)
			}
			if mailer.checked != "message-42" {
				t.Fatalf("checked message = %q, want message-42", mailer.checked)
			}
		})
	}
}
