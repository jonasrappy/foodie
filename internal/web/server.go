// Package web implements the public HTTP contract consumed by the existing APK.
package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jonasrappy/foodie/internal/auth"
	"github.com/jonasrappy/foodie/internal/config"
	"github.com/jonasrappy/foodie/internal/i18n"
	"github.com/jonasrappy/foodie/internal/store"
)

type Server struct {
	language          string
	timezone          string
	diagnosticLimiter *auth.Limiter
	voiceSlots        chan struct{}
	store             *store.Store
	auth              *auth.Auth
	limiter           *auth.Limiter
	loginSlots        chan struct{}
	publicDir         string
	hub               *hub
	logger            *slog.Logger
}

func New(db *store.Store, c config.Config, publicDir string, logger *slog.Logger) *Server {
	if c.Language == "" {
		c.Language = "en"
	}
	if c.Timezone == "" {
		c.Timezone = "UTC"
	}
	return &Server{language: c.Language, timezone: c.Timezone, diagnosticLimiter: auth.NewLimiter(), voiceSlots: make(chan struct{}, 1), store: db, auth: auth.New(c), limiter: auth.NewLimiter(), loginSlots: make(chan struct{}, 4), publicDir: publicDir, hub: newHub(), logger: logger}
}

var assets = map[string]string{"/": "index.html", "/robots.txt": "robots.txt", "/app.js": "app.js", "/foodie-3d.js": "foodie-3d.js", "/style.css": "style.css", "/manifest.webmanifest": "manifest.webmanifest", "/sw.js": "sw.js", "/icon.svg": "icon.svg", "/icon-192.png": "icon-192.png", "/icon-512.png": "icon-512.png", "/apple-touch-icon.png": "apple-touch-icon.png"}
var contentTypes = map[string]string{".html": "text/html; charset=utf-8", ".txt": "text/plain; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".webmanifest": "application/manifest+json", ".svg": "image/svg+xml", ".png": "image/png"}
var itemPath = regexp.MustCompile(`^/api/items/([a-f0-9]{32})$`)

func (s *Server) uiVersion() (string, error) {
	hash := sha256.New()
	hash.Write(i18n.JSON(s.language))
	for _, file := range []string{"index.html", "app.js", "style.css", "sw.js", "manifest.webmanifest", "foodie-3d.js"} {
		data, err := os.ReadFile(filepath.Join(s.publicDir, file))
		if err != nil {
			return "", err
		}
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil))[:20], nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logger.Error("HTTP handler panicked", "path", r.URL.Path, "panic_type", fmt.Sprintf("%T", recovered))
			http.Error(w, "Internal server error", 500)
		}
	}()
	if r.URL.Path == "/language.js" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != http.MethodHead {
			w.Write(append([]byte("window.FOODIE_I18N="), append(i18n.JSON(s.language), ';')...))
		}
		return
	}
	// SSE sets a fresh deadline per write; ordinary requests have a bounded lifetime.
	if r.URL.Path != "/api/events" {
		timeout := 15 * time.Second
		if isAndroidDownload(r.URL.Path) {
			timeout = 5 * time.Minute
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(timeout + 5*time.Second))
	}
	if file, ok := assets[r.URL.Path]; ok && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		data, err := os.ReadFile(filepath.Join(s.publicDir, file))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if file == "index.html" {
			version, err := s.uiVersion()
			if err != nil {
				s.fail(w, r, err)
				return
			}
			data = bytes.Replace(data, []byte("__APP_VERSION__"), []byte(version), 1)
			data = []byte(i18n.HTML(s.language, string(data)))
		}
		if file == "manifest.webmanifest" {
			var manifest map[string]any
			if err := json.Unmarshal(data, &manifest); err != nil {
				s.fail(w, r, err)
				return
			}
			manifest["lang"] = s.language
			manifest["description"] = i18n.Text(s.language, manifest["description"].(string))
			data, _ = json.Marshal(manifest)
		}
		w.Header().Set("Content-Type", contentTypes[filepath.Ext(file)])
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}
	switch {
	case r.URL.Path == "/health" && r.Method == http.MethodGet:
		if err := s.store.Ping(r.Context()); err != nil {
			s.fail(w, r, err)
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	case r.URL.Path == "/api/app-version" && r.Method == http.MethodGet:
		version, err := s.uiVersion()
		if err != nil {
			s.fail(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"version": version})
		return
	case r.URL.Path == "/api/login" && r.Method == http.MethodPost:
		s.login(w, r)
		return
	}
	if r.URL.Path == "/api/android/release" {
		s.androidRelease(w, r)
		return
	}
	if r.URL.Path == "/api/download/android/session" {
		s.androidDownloadSession(w, r)
		return
	}
	if isAndroidDownload(r.URL.Path) {
		s.androidDownload(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		s.problem(w, 404, "Siden findes ikke.")
		return
	}
	role := s.auth.Authenticate(r.Header.Get("Authorization"))
	if role == auth.Unauthenticated {
		s.problem(w, 401, "Log ind med husets kode.")
		return
	}
	if r.URL.Path == "/api/v1/requirements" && r.Method == http.MethodGet {
		state, err := s.store.Snapshot(r.Context())
		if err != nil {
			s.fail(w, r, err)
			return
		}
		respond(w, 200, s.requirements(state))
		return
	}
	if r.URL.Path == "/api/v1/reset" && r.Method == http.MethodPost {
		var input struct {
			Revision *int64 `json:"revision"`
		}
		if err := decode(w, r, &input); err != nil {
			s.fail(w, r, err)
			return
		}
		if input.Revision == nil || *input.Revision < 0 || *input.Revision > 9007199254740991 {
			s.problem(w, 400, "Angiv revision fra GET /api/v1/requirements.")
			return
		}
		state, err := s.store.Reset(r.Context(), *input.Revision)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		s.hub.publish(state)
		respond(w, 200, struct {
			OK bool `json:"ok"`
			store.State
		}{true, state})
		return
	}
	if role != auth.Device {
		s.problem(w, 403, "Bot-token kan kun hente krav og nulstille listerne.")
		return
	}
	switch {
	case r.URL.Path == "/api/voice/text" && r.Method == http.MethodPost:
		s.voiceCommand(w, r)
	case r.URL.Path == "/api/voice/diagnostic" && r.Method == http.MethodPost:
		s.voiceDiagnostic(w, r)
	case r.URL.Path == "/api/state" && r.Method == http.MethodGet:
		state, err := s.store.Snapshot(r.Context())
		if err != nil {
			s.fail(w, r, err)
			return
		}
		respond(w, 200, state)
	case r.URL.Path == "/api/events" && r.Method == http.MethodGet:
		s.events(w, r)
	case r.URL.Path == "/api/items" && r.Method == http.MethodPost:
		var input store.Add
		if err := decode(w, r, &input); err != nil {
			s.fail(w, r, err)
			return
		}
		state, err := s.store.Add(r.Context(), input)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		s.hub.publish(state)
		respond(w, 200, state)
	default:
		match := itemPath.FindStringSubmatch(r.URL.Path)
		if match == nil || (r.Method != http.MethodPatch && r.Method != http.MethodDelete) {
			s.problem(w, 404, "Endpoint findes ikke.")
			return
		}
		// Raw optional fields distinguish absent from JSON null, which is invalid.
		var input struct {
			Version  *int64          `json:"version"`
			Text     json.RawMessage `json:"text"`
			Checked  json.RawMessage `json:"checked"`
			Quantity json.RawMessage `json:"quantity"`
			Unit     json.RawMessage `json:"unit"`
		}
		if err := decode(w, r, &input); err != nil {
			s.fail(w, r, err)
			return
		}
		if input.Version == nil || *input.Version < 1 || *input.Version > 9007199254740991 {
			s.problem(w, 400, "Angiv linjens version.")
			return
		}
		edit := store.Edit{Version: *input.Version}
		if r.Method == http.MethodPatch {
			if input.Text != nil {
				var text string
				if string(input.Text) == "null" || json.Unmarshal(input.Text, &text) != nil {
					s.problem(w, 400, "Ugyldig tekst.")
					return
				}
				edit.Text = &text
			}

			if input.Quantity != nil {
				var quantity float64
				if string(input.Quantity) == "null" || json.Unmarshal(input.Quantity, &quantity) != nil {
					s.problem(w, 400, "Ugyldigt antal.")
					return
				}
				edit.Quantity = &quantity
			}
			if input.Unit != nil {
				var unit string
				if string(input.Unit) == "null" || json.Unmarshal(input.Unit, &unit) != nil {
					s.problem(w, 400, "Ugyldig enhed.")
					return
				}
				edit.Unit = &unit
			}
			if input.Checked != nil {
				var checked bool
				if string(input.Checked) == "null" || json.Unmarshal(input.Checked, &checked) != nil {
					s.problem(w, 400, "Ugyldig afkrydsning.")
					return
				}
				edit.Checked = &checked
			}
		}
		state, err := s.store.Edit(r.Context(), match[1], edit, r.Method == http.MethodDelete)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		s.hub.publish(state)
		respond(w, 200, state)
	}
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Listen is restricted to loopback. Trust the proxy header only on that hop.
	if parsed := net.ParseIP(ip); parsed != nil && parsed.IsLoopback() {
		if forwarded := net.ParseIP(r.Header.Get("X-Real-IP")); forwarded != nil {
			ip = forwarded.String()
		}
	}
	if !s.limiter.Allow(ip, time.Now()) {
		w.Header().Set("Retry-After", "900")
		s.problem(w, 429, "For mange forsøg. Prøv igen om 15 minutter.")
		return
	}
	var input struct {
		Password *string `json:"password"`
	}
	if err := decode(w, r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	if input.Password == nil || auth.TextLength(*input.Password) > 200 {
		s.problem(w, 401, "Koden er ikke rigtig. Prøv igen.")
		return
	}
	select {
	case s.loginSlots <- struct{}{}:
		defer func() { <-s.loginSlots }()
	default:
		w.Header().Set("Retry-After", "2")
		s.problem(w, 503, "Prøv igen om et øjeblik.")
		return
	}
	if !s.auth.CheckPassword(*input.Password) {
		s.problem(w, 401, "Koden er ikke rigtig. Prøv igen.")
		return
	}
	token, err := s.auth.IssueToken()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.limiter.Success(ip)
	respond(w, 200, map[string]string{"token": token})
}

type requirementsResponse struct {
	ShoppingItems         []store.Item `json:"shopping_items"`
	Revision              int64        `json:"revision"`
	UpdatedAt             string       `json:"updated_at"`
	Timezone              string       `json:"timezone"`
	RequiredMeals         []string     `json:"required_meals"`
	RequiredShoppingItems []string     `json:"required_shopping_items"`
	AlreadyPurchasedItems []string     `json:"already_purchased_items"`
	Instructions          string       `json:"instructions"`
}

func (s *Server) requirements(state store.State) requirementsResponse {
	result := requirementsResponse{ShoppingItems: state.Shopping, Revision: state.Revision, UpdatedAt: state.UpdatedAt, Timezone: s.timezone, RequiredMeals: []string{}, RequiredShoppingItems: []string{}, AlreadyPurchasedItems: []string{}, Instructions: "Retterne skal med i næste uges madplan. required_shopping_items skal købes. already_purchased_items er allerede købt og må ikke købes igen, heller ikke som ingredienser fra madplanen. Find selv resten. Nulstil først efter gennemført bestilling, med revisionen fra dette svar."}
	result.Instructions = i18n.Text(s.language, result.Instructions)
	for _, item := range state.Meals {
		result.RequiredMeals = append(result.RequiredMeals, item.Text)
	}
	for _, item := range state.Shopping {
		if item.Checked {
			result.AlreadyPurchasedItems = append(result.AlreadyPurchasedItems, s.shoppingLabel(item))
		} else {
			result.RequiredShoppingItems = append(result.RequiredShoppingItems, s.shoppingLabel(item))
		}
	}
	return result
}
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	ch, ok := s.hub.subscribe()
	if !ok {
		s.problem(w, 503, "For mange forbindelser.")
		return
	}
	defer s.hub.unsubscribe(ch)
	// Subscribe before snapshot so no commit can fall into a registration gap.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	state, err := s.store.Snapshot(ctx)
	cancel()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	send := func(data []byte) error {
		if err := controller.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		return controller.Flush()
	}
	data, err := json.Marshal(state)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err = send(append(append([]byte("data: "), data...), '\n', '\n')); err != nil {
		return
	}
	last := state.Revision
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if send([]byte(": ping\n\n")) != nil {
				return
			}
		case ev := <-ch:
			if ev.revision <= last {
				continue
			}
			last = ev.revision
			if send(append(append([]byte("data: "), ev.data...), '\n', '\n')) != nil {
				return
			}
		}
	}
}

type httpError struct {
	status  int
	message string
}

func (e httpError) Error() string { return e.message }
func decode(w http.ResponseWriter, r *http.Request, target any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return httpError{415, "Brug application/json."}
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return httpError{413, "For mange data."}
		}
		return httpError{400, "Kunne ikke læse forespørgslen."}
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return httpError{400, "Ugyldig JSON."}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return httpError{400, "Ugyldig JSON eller ukendte felter."}
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return httpError{400, "Ugyldig JSON."}
	}
	return nil
}
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var invalid store.Invalid
	var conflict store.Conflict
	var httpErr httpError
	switch {
	case errors.As(err, &invalid):
		s.problem(w, 400, invalid.Error())
	case errors.As(err, &conflict):
		s.problem(w, 409, conflict.Error())
	case errors.As(err, &httpErr):
		s.problem(w, httpErr.status, httpErr.message)
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		s.problem(w, 503, "Serveren er optaget. Prøv igen.")
	default:
		s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		s.problem(w, 500, "Der skete en fejl. Prøv igen.")
	}
}
func respond(w http.ResponseWriter, status int, data any) {
	// Encode before headers, so encoding failures never produce a partial success.
	encoded, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}
func (s *Server) problem(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": i18n.Text(s.language, message)})
}

func (s *Server) shoppingLabel(item store.Item) string {
	if item.Quantity == 1 && item.Unit == "stk." {
		return item.Text
	}
	return strconv.FormatFloat(item.Quantity, 'f', -1, 64) + " " + i18n.Unit(s.language, item.Unit, item.Quantity) + " " + item.Text
}
