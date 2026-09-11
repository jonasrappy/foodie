package web

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jonasrappy/foodie/internal/auth"
)

const androidCookie = "__Secure-mad_apk"
const androidFilename = "foodie.apk"

func isAndroidDownload(path string) bool {
	return path == "/api/download/android" || path == "/downloads/foodie.apk"
}

// Prepare a normal browser download. Android's download manager can send this
// short-lived cookie; it cannot inherit an Authorization header from fetch().
func (s *Server) androidDownloadSession(w http.ResponseWriter, r *http.Request) {
	role := s.auth.Authenticate(r.Header.Get("Authorization"))
	if role == auth.Unauthenticated {
		s.problem(w, 401, "Log ind for at hente Android-appen.")
		return
	}
	if role != auth.Device {
		s.problem(w, 403, "APK-download kræver login i appen.")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		s.problem(w, 405, "Brug POST til at forberede download.")
		return
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.publicDir), "downloads", "foodie.apk")); err != nil {
		s.fail(w, r, err)
		return
	}
	now := time.Now()
	grant := s.auth.IssueDownloadToken(now)
	http.SetCookie(w, &http.Cookie{
		Name: androidCookie, Value: grant,
		Path: "/api/download/android", MaxAge: int(auth.DownloadLifetime.Seconds()),
		Expires: now.Add(auth.DownloadLifetime), Secure: true, HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	respond(w, 200, map[string]any{
		"expires_in": int(auth.DownloadLifetime.Seconds()), "filename": androidFilename,
		"url": "/api/download/android?grant=" + grant,
	})
}

func (s *Server) androidDownload(w http.ResponseWriter, r *http.Request) {
	role := s.auth.Authenticate(r.Header.Get("Authorization"))
	if role == auth.Unauthenticated && r.Header.Get("Authorization") == "" && r.URL.Path == "/api/download/android" {
		if cookie, err := r.Cookie(androidCookie); err == nil && s.auth.CheckDownloadToken(cookie.Value, time.Now()) {
			role = auth.Device
		}
		// Explicit direct links use the same expiring, APK-only grant. Android
		// can open them in another browser without sharing its login cookies.
		if s.auth.CheckDownloadToken(r.URL.Query().Get("grant"), time.Now()) {
			role = auth.Device
		}
	}
	if role == auth.Unauthenticated {
		s.problem(w, 401, "Log ind for at hente Android-appen.")
		return
	}
	if role != auth.Device {
		s.problem(w, 403, "APK-download kræver login i appen.")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.problem(w, 405, "Brug GET til at hente appen.")
		return
	}
	file, err := os.Open(filepath.Join(filepath.Dir(s.publicDir), "downloads", "foodie.apk"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", `attachment; filename="`+androidFilename+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Vary", "Cookie, Authorization")
	http.ServeContent(w, r, androidFilename, info.ModTime(), file)
}
