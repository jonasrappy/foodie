package auth

import (
	"strconv"
	"strings"
	"time"
)

// Download cookies authorize only APK reads, never household API access. Unlike
// the device token, the signed expiry is enforced even if the cookie is copied.
const DownloadLifetime = 10 * time.Minute

func (a *Auth) IssueDownloadToken(now time.Time) string {
	expires := strconv.FormatInt(now.Add(DownloadLifetime).Unix(), 10)
	return "download." + expires + "." + a.signature("android-download:"+expires)
}

func (a *Auth) CheckDownloadToken(token string, now time.Time) bool {
	if len(token) > 256 {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "download" {
		return false
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && expires > now.Unix() &&
		expires <= now.Add(DownloadLifetime).Unix() &&
		constantEqual(parts[2], a.signature("android-download:"+parts[1]))
}
