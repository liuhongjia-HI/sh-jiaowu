package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"
	"sync"
	"time"
)

var (
	ErrLoginLocked      = errors.New("登录失败次数过多，请稍后再试")
	ErrLoginRateLimited = errors.New("登录太频繁，请稍后再试")
)

type LoginProtector struct {
	mu          sync.Mutex
	now         func() time.Time
	maxFailures int
	lockFor     time.Duration
	window      time.Duration
	maxAttempts int
	records     map[string]*loginRecord
	captchas    map[string]captchaChallenge
}

type loginRecord struct {
	failures    int
	lockedUntil time.Time
	attempts    []time.Time
}

type captchaChallenge struct {
	Answer    string
	ExpiresAt time.Time
}

type Captcha struct {
	ID        string `json:"captchaId"`
	Question  string `json:"question"`
	ExpiresIn int    `json:"expiresIn"`
}

func NewLoginProtector() *LoginProtector {
	return &LoginProtector{
		now:         time.Now,
		maxFailures: 5,
		lockFor:     15 * time.Minute,
		window:      time.Minute,
		maxAttempts: 20,
		records:     map[string]*loginRecord{},
		captchas:    map[string]captchaChallenge{},
	}
}

func (p *LoginProtector) Allow(key string) error {
	if p == nil || key == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	record := p.record(key)
	if now.Before(record.lockedUntil) {
		return ErrLoginLocked
	}
	record.attempts = recentAttempts(record.attempts, now.Add(-p.window))
	if len(record.attempts) >= p.maxAttempts {
		return ErrLoginRateLimited
	}
	record.attempts = append(record.attempts, now)
	return nil
}

func (p *LoginProtector) RegisterSuccess(key string) {
	if p == nil || key == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.records, key)
}

func (p *LoginProtector) RegisterFailure(key string) {
	if p == nil || key == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	record := p.record(key)
	record.failures++
	if record.failures >= p.maxFailures {
		record.lockedUntil = now.Add(p.lockFor)
	}
}

func (p *LoginProtector) RequiresCaptcha(key string) bool {
	if p == nil || key == "" {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	record := p.record(key)
	return record.failures >= 3
}

func (p *LoginProtector) NewCaptcha() (Captcha, error) {
	if p == nil {
		return Captcha{}, nil
	}
	a, err := secureDigit()
	if err != nil {
		return Captcha{}, err
	}
	b, err := secureDigit()
	if err != nil {
		return Captcha{}, err
	}
	id, err := randomID()
	if err != nil {
		return Captcha{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.captchas[id] = captchaChallenge{Answer: strconv.Itoa(a + b), ExpiresAt: p.now().Add(5 * time.Minute)}
	return Captcha{ID: id, Question: strconv.Itoa(a) + " + " + strconv.Itoa(b) + " = ?", ExpiresIn: 300}, nil
}

func (p *LoginProtector) VerifyCaptcha(id, answer string) bool {
	if p == nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	challenge, ok := p.captchas[id]
	if !ok || p.now().After(challenge.ExpiresAt) || challenge.Answer != answer {
		return false
	}
	delete(p.captchas, id)
	return true
}

func (p *LoginProtector) record(key string) *loginRecord {
	record := p.records[key]
	if record == nil {
		record = &loginRecord{}
		p.records[key] = record
	}
	return record
}

func secureDigit() (int, error) {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int(b[0]%8) + 1, nil
}

func randomID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func recentAttempts(items []time.Time, cutoff time.Time) []time.Time {
	next := items[:0]
	for _, item := range items {
		if item.After(cutoff) {
			next = append(next, item)
		}
	}
	return next
}
