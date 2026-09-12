package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/jonasrappy/foodie/internal/config"
)

func TestDownloadTokenExpiryScopeAndRotation(t *testing.T) {
	c := config.Config{SessionSecret: "fixture-secret", PasswordHash: "fixture-hash"}
	a := New(c)
	now := time.Unix(1800000000, 0)
	token := a.IssueDownloadToken(now)
	if !a.CheckDownloadToken(token, now) || !a.CheckDownloadToken(token, now.Add(DownloadLifetime-time.Second)) {
		t.Fatal("valid APK grant rejected")
	}
	if a.CheckDownloadToken(token, now.Add(DownloadLifetime)) || a.CheckDownloadToken(token, now.Add(24*time.Hour)) {
		t.Fatal("expired APK grant accepted")
	}
	parts := strings.Split(token, ".")
	for _, invalid := range []string{"", token + "x", "download.1900000000." + parts[2], "download.bad." + parts[2], strings.Repeat("x", 1000)} {
		if a.CheckDownloadToken(invalid, now) {
			t.Fatal("invalid APK grant accepted")
		}
	}
	if a.Authenticate("Bearer "+token) != Unauthenticated || a.Authenticate("Bearer device."+parts[1]+"."+parts[2]) != Unauthenticated {
		t.Fatal("APK grant authorized household API access")
	}
	device, err := a.IssueToken()
	if err != nil || a.CheckDownloadToken(device, now) {
		t.Fatal("device token accepted as an expiring download cookie")
	}
	c.PasswordHash = "rotated-password"
	if New(c).CheckDownloadToken(token, now) {
		t.Fatal("password rotation did not revoke download grants")
	}
}
