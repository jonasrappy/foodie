// Package auth preserves non-expiring device and bot tokens issued by the Node app.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/jonasrappy/foodie/internal/config"
)

type Role int

const (
	Unauthenticated Role = iota
	Device
	Bot
)

type Auth struct{ config config.Config }

func New(c config.Config) *Auth { return &Auth{config: c} }
func HashPassword(password, salt string) (string, error) {
	return config.DerivePassword(password, salt)
}
func TextLength(s string) int { return len(utf16.Encode([]rune(s))) }
func (a *Auth) CheckPassword(password string) bool {
	if TextLength(password) > 200 {
		return false
	}
	hash, err := HashPassword(password, a.config.PasswordSalt)
	return err == nil && constantEqual(hash, a.config.PasswordHash)
}
func (a *Auth) signature(nonce string) string {
	// These are concatenated ASCII strings, not decoded hexadecimal keys.
	mac := hmac.New(sha256.New, []byte(a.config.SessionSecret+a.config.PasswordHash))
	mac.Write([]byte(nonce))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (a *Auth) IssueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	nonce := base64.RawURLEncoding.EncodeToString(b)
	return "device." + nonce + "." + a.signature(nonce), nil
}
func (a *Auth) Authenticate(header string) Role {
	if !strings.HasPrefix(header, "Bearer ") {
		return Unauthenticated
	}
	token := strings.TrimPrefix(header, "Bearer ")
	if token == "" || len(token) > 1024 {
		return Unauthenticated
	}
	hash := sha256.Sum256([]byte(token))
	if constantEqual(hex.EncodeToString(hash[:]), a.config.BotTokenHash) {
		return Bot
	}
	parts := strings.Split(token, ".")
	if len(parts) == 3 && parts[0] == "device" && parts[1] != "" && constantEqual(parts[2], a.signature(parts[1])) {
		return Device
	}
	return Unauthenticated
}
func constantEqual(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

type attempt struct {
	count int
	until time.Time
}
type Limiter struct {
	mu          sync.Mutex
	entries     map[string]attempt
	lastCleanup time.Time
}

func NewLimiter() *Limiter { return &Limiter{entries: make(map[string]attempt)} }
func (l *Limiter) Allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastCleanup) > time.Minute {
		for ip, a := range l.entries {
			if !now.Before(a.until) {
				delete(l.entries, ip)
			}
		}
		l.lastCleanup = now
	}
	a, ok := l.entries[ip]
	if !ok || !now.Before(a.until) {
		if !ok && len(l.entries) >= 4096 {
			return false
		}
		a = attempt{until: now.Add(15 * time.Minute)}
	}
	a.count++
	l.entries[ip] = a
	return a.count <= 15
}
func (l *Limiter) Success(ip string) { l.mu.Lock(); defer l.mu.Unlock(); delete(l.entries, ip) }
