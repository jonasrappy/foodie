package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAndroidRelease(t *testing.T) {
	s, _, f := testServer(t)
	root := t.TempDir()
	s.publicDir = filepath.Join(root, "public")
	if err := os.Mkdir(filepath.Join(root, "downloads"), 0755); err != nil {
		t.Fatal(err)
	}
	publish := func(metadata string) {
		t.Helper()
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		file, err := z.Create("assets/release.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write([]byte(metadata)); err != nil {
			t.Fatal(err)
		}
		if err = z.Close(); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, "downloads", androidFilename), b.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	call := func(method, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://foodie.example.com/api/android/release", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {f.Bot, 403}, {f.Token, 404}} {
		if w := call("GET", tc.token); w.Code != tc.status {
			t.Fatalf("status %d expected %d", w.Code, tc.status)
		}
	}
	publish(`{"version_code":8,"version_name":"1.3.2"}`)
	if call("POST", f.Token).Code != 405 {
		t.Fatal("method not restricted")
	}
	for _, code := range []int{8, 9} {
		if code == 9 {
			publish(`{"version_code":9,"version_name":"1.3.3"}`)
		}
		w := call("GET", f.Token)
		var release struct {
			Code int `json:"version_code"`
		}
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(w.Body.Bytes(), &release) != nil || release.Code != code {
			t.Fatal("release does not follow published APK")
		}
	}
	for _, invalid := range []string{`{"version_code":0,"version_name":"1.3.2"}`, `{"version_code":8,"version_name":"bad"}`, `{`, `{"version_code":2100000000,"version_name":"1.3.2"}`} {
		publish(invalid)
		if call("GET", f.Token).Code != 500 {
			t.Fatal("invalid release metadata accepted")
		}
	}
}
