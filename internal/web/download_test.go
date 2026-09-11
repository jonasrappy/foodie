package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonasrappy/foodie/internal/auth"
)

func TestAndroidBrowserDownload(t *testing.T) {
	s, _, f := testServer(t)
	root := t.TempDir()
	s.publicDir = filepath.Join(root, "public")
	if err := os.Mkdir(filepath.Join(root, "downloads"), 0755); err != nil {
		t.Fatal(err)
	}
	apk := []byte("PK\x03\x04browser-download-fixture")
	if err := os.WriteFile(filepath.Join(root, "downloads", "til-bordet.apk"), apk, 0644); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, token string, cookie *http.Cookie, byteRange string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://foodie.example.com"+path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if byteRange != "" {
			r.Header.Set("Range", byteRange)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {f.Bot, 403}} {
		w := call("POST", "/api/download/android/session", tc.token, nil, "")
		if w.Code != tc.status || w.Header().Get("Set-Cookie") != "" {
			t.Fatal("unauthorized client obtained a download grant")
		}
	}
	w := call("POST", "/api/download/android/session", f.Token, nil, "")
	cookies := w.Result().Cookies()
	if w.Code != 200 || len(cookies) != 1 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("could not prepare private browser download")
	}
	cookie := cookies[0]
	var prepared struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &prepared); err != nil || prepared.URL != "/api/download/android?grant="+cookie.Value {
		t.Fatal("session did not return a scoped direct download link")
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/api/download/android" || cookie.MaxAge != 600 {
		t.Fatal("download cookie is not scoped or expiring")
	}
	w = call("GET", "/api/download/android?t=123456", "", cookie, "")
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), apk) || w.Header().Get("Content-Disposition") != `attachment; filename="foodie.apk"` || w.Header().Get("Content-Type") != "application/vnd.android.package-archive" || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("browser did not receive the versioned APK attachment")
	}
	w = call("HEAD", "/api/download/android", "", cookie, "")
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Fatal("download preparation fetched APK bytes")
	}
	w = call("GET", "/api/download/android", "", cookie, "bytes=4-10")
	if w.Code != 206 || !bytes.Equal(w.Body.Bytes(), apk[4:11]) || !strings.HasPrefix(w.Header().Get("Content-Range"), "bytes 4-10/") {
		t.Fatal("Android download resume is broken")
	}
	// A direct link must also work in a fresh browser without login cookies.
	link := "/api/download/android?grant=" + cookie.Value + "&t=123456"
	w = call("GET", link, "", nil, "")
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), apk) || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("direct APK link requires browser cookies or leaks its referrer")
	}
	if call("GET", link, "", nil, "bytes=4-10").Code != 206 {
		t.Fatal("direct APK link cannot resume downloads")
	}
	if call("GET", link, f.Bot, nil, "").Code != 403 {
		t.Fatal("direct link bypassed bot restrictions")
	}
	for _, path := range []string{"/api/state", "/api/v1/requirements", "/downloads/til-bordet.apk"} {
		if call("GET", path+"?grant="+cookie.Value, "", nil, "").Code != 401 {
			t.Fatal("APK link granted unrelated access")
		}
	}
	for _, path := range []string{"/api/state", "/api/v1/requirements", "/downloads/til-bordet.apk", "/api/download/android/session"} {
		if call("GET", path, "", cookie, "").Code != 401 {
			t.Fatalf("APK cookie granted access to %s", path)
		}
	}
	if call("POST", "/api/download/android/session", "", cookie, "").Code != 401 {
		t.Fatal("APK cookie refreshed itself without device login")
	}
	if call("GET", "/api/download/android", f.Bot, cookie, "").Code != 403 {
		t.Fatal("cookie bypassed bot download restriction")
	}
	if call("POST", "/api/download/android", "", cookie, "").Code != 405 {
		t.Fatal("unsupported download method accepted")
	}
	cookie.Value = s.auth.IssueDownloadToken(time.Now().Add(-auth.DownloadLifetime))
	if call("GET", "/api/download/android", "", cookie, "").Code != 401 {
		t.Fatal("expired browser grant accepted")
	}
	if call("GET", "/api/download/android?grant="+cookie.Value, "", nil, "").Code != 401 {
		t.Fatal("expired direct APK link accepted")
	}
	if call("GET", "/api/download/android?grant=invalid", "", nil, "").Code != 401 {
		t.Fatal("invalid direct APK link accepted")
	}
}
