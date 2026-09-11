package auth

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonasrappy/foodie/internal/config"
)

func TestNodeCompatibilityAndPasswordRotation(t *testing.T) {
	var fixture struct {
		Password, Bot, Token string
		Config               config.Config
	}
	data, err := os.ReadFile("testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	a := New(fixture.Config)
	if !a.CheckPassword(fixture.Password) || a.CheckPassword("wrong") {
		t.Fatal("Node scrypt compatibility failed")
	}
	if a.Authenticate("Bearer "+fixture.Token) != Device {
		t.Fatal("existing Node device token rejected")
	}
	if a.Authenticate("Bearer "+fixture.Bot) != Bot {
		t.Fatal("existing bot token rejected")
	}
	for _, bad := range []string{"", fixture.Token, "Bearer " + fixture.Token + "x", "Bearer " + fixture.Token + ".extra", "Bearer " + strings.Repeat("x", 2000)} {
		if a.Authenticate(bad) != Unauthenticated {
			t.Fatalf("accepted invalid token %q", bad)
		}
	}
	issued, err := a.IssueToken()
	if err != nil {
		t.Fatal(err)
	}
	if a.Authenticate("Bearer "+issued) != Device {
		t.Fatal("issued token rejected")
	}
	fixture.Config.PasswordHash, err = HashPassword("new-password", fixture.Config.PasswordSalt)
	if err != nil {
		t.Fatal(err)
	}
	rotated := New(fixture.Config)
	if rotated.Authenticate("Bearer "+fixture.Token) != Unauthenticated || rotated.Authenticate("Bearer "+issued) != Unauthenticated {
		t.Fatal("password change did not revoke sessions")
	}
	if rotated.Authenticate("Bearer "+fixture.Bot) != Bot {
		t.Fatal("password rotation revoked bot")
	}
}
func TestConcurrentLimiterAndWindow(t *testing.T) {
	limiter := NewLimiter()
	now := time.Now()
	var wg sync.WaitGroup
	allowed := make(chan bool, 100)
	for range 100 {
		wg.Go(func() { allowed <- limiter.Allow("client", now) })
	}
	wg.Wait()
	close(allowed)
	n := 0
	for ok := range allowed {
		if ok {
			n++
		}
	}
	if n != 15 {
		t.Fatalf("allowed %d attempts", n)
	}
	if !limiter.Allow("client", now.Add(16*time.Minute)) {
		t.Fatal("window did not expire")
	}
	limiter.Success("client")
	if !limiter.Allow("client", now) {
		t.Fatal("success did not clear limiter")
	}
}
