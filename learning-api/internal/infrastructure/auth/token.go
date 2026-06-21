package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"starline/learning-api/internal/domain/learning"
)

type TokenManager struct {
	secret  []byte
	ttl     time.Duration
	mu      sync.Mutex
	revoked map[string]time.Time
}

type tokenPayload struct {
	learning.Principal
	TokenID   string `json:"tokenId"`
	ExpiresAt int64  `json:"expiresAt"`
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	if strings.TrimSpace(secret) == "" {
		secret = "starline-local-dev-secret"
	}
	return &TokenManager{secret: []byte(secret), ttl: ttl, revoked: map[string]time.Time{}}
}

func (m *TokenManager) Issue(principal learning.Principal) (string, error) {
	tokenID, err := randomTokenID()
	if err != nil {
		return "", err
	}
	payload := tokenPayload{
		Principal: principal,
		TokenID:   tokenID,
		ExpiresAt: time.Now().Add(m.ttl).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	return body + "." + m.sign(body), nil
}

func (m *TokenManager) Parse(token string) (learning.Principal, error) {
	body, signature, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || body == "" || signature == "" {
		return learning.Principal{}, errors.New("invalid token")
	}
	if !hmac.Equal([]byte(signature), []byte(m.sign(body))) {
		return learning.Principal{}, errors.New("invalid token signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return learning.Principal{}, errors.New("invalid token payload")
	}
	var payload tokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return learning.Principal{}, errors.New("invalid token payload")
	}
	if payload.ExpiresAt < time.Now().Unix() {
		return learning.Principal{}, errors.New("token expired")
	}
	if m.isRevoked(payload.TokenID) {
		return learning.Principal{}, errors.New("token revoked")
	}
	return payload.Principal, nil
}

func (m *TokenManager) Revoke(token string) error {
	body, signature, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || body == "" || signature == "" {
		return errors.New("invalid token")
	}
	if !hmac.Equal([]byte(signature), []byte(m.sign(body))) {
		return errors.New("invalid token signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return errors.New("invalid token payload")
	}
	var payload tokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return errors.New("invalid token payload")
	}
	if payload.TokenID == "" {
		return errors.New("invalid token payload")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked[payload.TokenID] = time.Unix(payload.ExpiresAt, 0)
	m.cleanupRevokedLocked(time.Now())
	return nil
}

func (m *TokenManager) isRevoked(tokenID string) bool {
	if tokenID == "" {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.cleanupRevokedLocked(now)
	expiresAt, ok := m.revoked[tokenID]
	return ok && expiresAt.After(now)
}

func (m *TokenManager) cleanupRevokedLocked(now time.Time) {
	for tokenID, expiresAt := range m.revoked {
		if !expiresAt.After(now) {
			delete(m.revoked, tokenID)
		}
	}
}

func (m *TokenManager) sign(value string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomTokenID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
