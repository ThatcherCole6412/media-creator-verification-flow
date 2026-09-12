package flow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

type Mailer interface {
	SendVerification(context.Context, string, string, string) (string, error)
	GetEmail(context.Context, string) (any, error)
}

type Account struct {
	ID                    string `json:"id"`
	Email                 string `json:"email"`
	VerificationMessageID string `json:"verification_message_id"`
	Verified              bool   `json:"verified"`
}

type Asset struct {
	ID        string `json:"id"`
	CreatorID string `json:"creator_id"`
	Source    string `json:"source"`
	State     string `json:"state"`
}

type ProcessingJob struct {
	ID      string `json:"id"`
	AssetID string `json:"asset_id"`
	State   string `json:"state"`
}

type Delivery struct {
	AssetID   string `json:"asset_id"`
	CreatorID string `json:"creator_id"`
	State     string `json:"state"`
}

type SignupResult struct {
	AccountID string `json:"account_id"`
	AssetID   string `json:"asset_id"`
	JobID     string `json:"job_id"`
	State     string `json:"state"`
}

type Service struct {
	mu       sync.Mutex
	mailer   Mailer
	baseURL  string
	accounts map[string]*Account
	tokens   map[string]string
	assets   map[string]*Asset
	jobs     map[string]*ProcessingJob
}

func NewService(mailer Mailer, baseURL string) *Service {
	return &Service{mailer: mailer, baseURL: baseURL, accounts: map[string]*Account{}, tokens: map[string]string{}, assets: map[string]*Asset{}, jobs: map[string]*ProcessingJob{}}
}

func (s *Service) Signup(ctx context.Context, email, source string) (SignupResult, error) {
	if email == "" || source == "" {
		return SignupResult{}, errors.New("email and source are required")
	}
	accountID, err := newID()
	if err != nil {
		return SignupResult{}, err
	}
	assetID, err := newID()
	if err != nil {
		return SignupResult{}, err
	}
	jobID, err := newID()
	if err != nil {
		return SignupResult{}, err
	}
	token, err := newID()
	if err != nil {
		return SignupResult{}, err
	}
	messageID, err := s.mailer.SendVerification(ctx, email, s.baseURL+"/verify?token="+token, "signup-"+accountID)
	if err != nil {
		return SignupResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[accountID] = &Account{ID: accountID, Email: email, VerificationMessageID: messageID}
	s.tokens[token] = accountID
	s.assets[assetID] = &Asset{ID: assetID, CreatorID: accountID, Source: source, State: "ingested"}
	s.jobs[jobID] = &ProcessingJob{ID: jobID, AssetID: assetID, State: "queued"}
	return SignupResult{AccountID: accountID, AssetID: assetID, JobID: jobID, State: "verification_pending"}, nil
}

func (s *Service) Verify(ctx context.Context, token string) (Delivery, error) {
	s.mu.Lock()
	accountID := s.tokens[token]
	account := s.accounts[accountID]
	s.mu.Unlock()
	if account == nil {
		return Delivery{}, errors.New("invalid verification token")
	}
	if _, err := s.mailer.GetEmail(ctx, account.VerificationMessageID); err != nil {
		return Delivery{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	account.Verified = true
	delete(s.tokens, token)
	for _, job := range s.jobs {
		asset := s.assets[job.AssetID]
		if asset.CreatorID == accountID {
			job.State = "completed"
			asset.State = "delivered"
			return Delivery{AssetID: asset.ID, CreatorID: accountID, State: "delivered"}, nil
		}
	}
	return Delivery{}, fmt.Errorf("no asset for creator %s", accountID)
}

func newID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
