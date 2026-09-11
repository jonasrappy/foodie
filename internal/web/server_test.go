package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonasrappy/foodie/internal/config"
	"github.com/jonasrappy/foodie/internal/store"
)

type fixture struct {
	Password, Bot, Token string
	Config               config.Config
}

func testPublicDir() string {
	if dir := os.Getenv("MAD_TEST_PUBLIC_DIR"); dir != "" {
		return dir
	}
	return "../../public"
}

func testServer(t *testing.T) (*Server, *httptest.Server, fixture) {
	t.Helper()
	var f fixture
	data, err := os.ReadFile("../auth/testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	f.Config.Language = "da"
	f.Config.Timezone = "Europe/Copenhagen"
	db, err := store.Open(filepath.Join(t.TempDir(), "mad.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(db, f.Config, testPublicDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	server := httptest.NewServer(s)
	t.Cleanup(func() { server.Close(); db.Close() })
	return s, server, f
}
func request(t *testing.T, url, method, body, token string) (int, []byte) {
	t.Helper()
	r, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, data
}
func stateResponse(t *testing.T, data []byte) store.State {
	t.Helper()
	var state store.State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("%s: %v", data, err)
	}
	return state
}
func TestHTTPContractAndAuthorization(t *testing.T) {
	_, server, f := testServer(t)
	for _, path := range []string{"/api/state", "/api/events", "/api/v1/requirements"} {
		if status, _ := request(t, server.URL+path, "GET", "", ""); status != 401 {
			t.Fatalf("unauthenticated %s: %d", path, status)
		}
	}
	if status, _ := request(t, server.URL+"/api/state", "GET", "", f.Bot); status != 403 {
		t.Fatal("bot can access device API")
	}
	if status, _ := request(t, server.URL+"/api/login", "POST", `{"password":"wrong"}`, ""); status != 401 {
		t.Fatal("wrong password accepted")
	}
	body, _ := json.Marshal(map[string]string{"password": f.Password})
	status, data := request(t, server.URL+"/api/login", "POST", string(body), "")
	if status != 200 {
		t.Fatalf("login: %d %s", status, data)
	}
	var login map[string]string
	if err := json.Unmarshal(data, &login); err != nil {
		t.Fatal(err)
	}
	status, data = request(t, server.URL+"/api/state", "GET", "", login["token"])
	if status != 200 {
		t.Fatal("new token rejected")
	}
	if !bytes.Contains(data, []byte(`"shopping":[]`)) || !bytes.Contains(data, []byte(`"meals":[]`)) {
		t.Fatalf("arrays must not be null: %s", data)
	}
	status, data = request(t, server.URL+"/api/items", "POST", `{"kind":"shopping","texts":["Mælk"],"request_id":"http-request-000001","quantity":1.5,"unit":"liter"}`, f.Token)
	if status != 200 {
		t.Fatalf("add: %d %s", status, data)
	}
	state := stateResponse(t, data)
	item := state.Shopping[0]
	status, data = request(t, server.URL+"/api/v1/requirements", "GET", "", f.Bot)
	if status != 200 {
		t.Fatal("export failed")
	}
	var req requirementsResponse
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	if len(req.RequiredShoppingItems) != 1 || req.RequiredShoppingItems[0] != "1.5 liter Mælk" || req.ShoppingItems[0].Quantity != 1.5 {
		t.Fatalf("quantity missing from bot export: %s", data)
	}
	status, data = request(t, server.URL+"/api/items/"+item.ID, "PATCH", `{"version":1,"checked":true,"quantity":500,"unit":"milliliter"}`, f.Token)
	if status != 200 {
		t.Fatalf("check: %d %s", status, data)
	}
	if status, _ = request(t, server.URL+"/api/items/"+item.ID, "PATCH", `{"version":1,"text":"stale"}`, f.Token); status != 409 {
		t.Fatal("stale edit accepted")
	}
	status, data = request(t, server.URL+"/api/v1/requirements", "GET", "", f.Bot)
	if status != 200 {
		t.Fatal("export failed")
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	if len(req.RequiredShoppingItems) != 0 || len(req.AlreadyPurchasedItems) != 1 || req.AlreadyPurchasedItems[0] != "500 milliliter Mælk" || !req.ShoppingItems[0].Checked {
		t.Fatalf("checked item export incorrect: %s", data)
	}
	status, data = request(t, server.URL+"/api/v1/reset", "POST", `{"revision":1}`, f.Bot)
	if status != 409 {
		t.Fatalf("stale reset: %d %s", status, data)
	}
	status, data = request(t, server.URL+"/api/v1/reset", "POST", fmt.Sprintf(`{"revision":%d}`, req.Revision), f.Bot)
	if status != 200 || !bytes.Contains(data, []byte(`"ok":true`)) {
		t.Fatalf("reset: %d %s", status, data)
	}
	state = stateResponse(t, data)
	if len(state.Shopping) != 0 {
		t.Fatal("reset not empty")
	}
	for _, path := range []string{"/robots.txt", "/", "/api/app-version", "/foodie-3d.js"} {
		status, data = request(t, server.URL+path, "GET", "", "")
		if status != 200 {
			t.Fatalf("public asset %s: %d", path, status)
		}
		if path == "/robots.txt" && !bytes.Contains(data, []byte("Disallow: /")) {
			t.Fatal("robots missing")
		}
	}
}
func TestRejectMalformedAndOversizedBodies(t *testing.T) {
	_, server, f := testServer(t)
	cases := []struct {
		body string
		want int
	}{
		{`null`, 400}, {`[]`, 400}, {`{}`, 400}, {`{"kind":"shopping","texts":["a"],"request_id":"invalid"}`, 400},
		{`{"kind":"shopping","texts":["a"],"request_id":"request-valid-0001","quantity":0}`, 400},
		{`{"kind":"shopping","texts":["a"],"request_id":"request-valid-0001","unit":"dl"}`, 400},
		{`{"kind":"shopping","texts":["a"],"request_id":"request-valid-0001","quantity":"2"}`, 400},
		{`{"kind":"shopping","texts":["a"],"request_id":"request-valid-0001","extra":true}`, 400},
		{`{"kind":"shopping","texts":["a"],"request_id":"request-valid-0001"}{}`, 400},
		{`{"kind":"shopping","texts":["` + strings.Repeat("x", 33000) + `"],"request_id":"request-valid-0001"}`, 413},
	}
	for i, test := range cases {
		status, data := request(t, server.URL+"/api/items", "POST", test.body, f.Token)
		if status != test.want {
			t.Fatalf("case %d: %d %s", i, status, data)
		}
	}
	r, _ := http.NewRequestWithContext(t.Context(), "POST", server.URL+"/api/items", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+f.Token)
	r.Header.Set("Content-Type", "text/plain")
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 415 {
		t.Fatal("wrong content type accepted")
	}
	for _, body := range []string{`{}`, `{"revision":null}`, `{"revision":1.5}`, `{"revision":-1}`, `{"revision":9007199254740992}`} {
		if status, _ := request(t, server.URL+"/api/v1/reset", "POST", body, f.Bot); status != 400 {
			t.Fatalf("bad revision accepted: %s", body)
		}
	}
}
func TestSSEConcurrentUpdatesAndDisconnect(t *testing.T) {
	s, server, f := testServer(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events", nil)
	r.Header.Set("Authorization", "Bearer "+f.Token)
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("SSE not established")
	}
	states := make(chan store.State, 100)
	readErrors := make(chan error, 1)
	go func() {
		defer close(states)
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 4096), 2*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				var state store.State
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &state); err != nil {
					readErrors <- err
					return
				}
				states <- state
			}
		}
		if err := scanner.Err(); err != nil {
			readErrors <- err
		}
	}()
	first := <-states
	if first.Revision != 0 {
		t.Fatal("wrong initial snapshot")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 25)
	for i := range 25 {
		wg.Go(func() {
			state, err := s.store.Add(ctx, store.Add{Kind: "shopping", Texts: []string{fmt.Sprint(i)}, RequestID: fmt.Sprintf("sse-request-%08d", i)})
			if err == nil {
				s.hub.publish(state)
			}
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	last := int64(0)
	for last < 25 {
		select {
		case state, ok := <-states:
			if !ok {
				t.Fatal("stream closed early")
			}
			if state.Revision <= last {
				t.Fatalf("out of order: %d after %d", state.Revision, last)
			}
			if len(state.Shopping) != int(state.Revision) {
				t.Fatal("revision and contents disagree")
			}
			last = state.Revision
		case err := <-readErrors:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal("SSE timed out")
		}
	}
	cancel()
	response.Body.Close()
	deadline := time.Now().Add(time.Second)
	for {
		s.hub.mu.Lock()
		n := len(s.hub.clients)
		s.hub.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnected SSE subscription leaked")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestHubCoalescesAndCapsClients(t *testing.T) {
	h := newHub()
	slow, ok := h.subscribe()
	if !ok {
		t.Fatal("subscribe")
	}
	for i := int64(0); i < 1000; i++ {
		h.publish(store.State{Revision: i})
	}
	if got := (<-slow).revision; got != 999 {
		t.Fatalf("slow client retained old state %d", got)
	}
	h.publish(store.State{Revision: 1})
	if len(slow) != 0 {
		t.Fatal("published stale state")
	}
	for range 99 {
		if _, ok = h.subscribe(); !ok {
			t.Fatal("early capacity limit")
		}
	}
	if _, ok = h.subscribe(); ok {
		t.Fatal("subscriber limit ignored")
	}
	h.unsubscribe(slow)
	if _, ok = h.subscribe(); !ok {
		t.Fatal("slot not released")
	}
}

func TestAPKDownloadRequiresDeviceLogin(t *testing.T) {
	s, server, f := testServer(t)
	root := t.TempDir()
	s.publicDir = filepath.Join(root, "public")
	if err := os.MkdirAll(filepath.Join(root, "downloads"), 0755); err != nil {
		t.Fatal(err)
	}
	apk := []byte("PK\x03\x04test-apk")
	if err := os.WriteFile(filepath.Join(root, "downloads", "foodie.apk"), apk, 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/download/android", "/downloads/foodie.apk"} {
		if status, _ := request(t, server.URL+path, "GET", "", ""); status != 401 {
			t.Fatalf("public APK download: %d", status)
		}
		if status, _ := request(t, server.URL+path, "GET", "", f.Bot); status != 403 {
			t.Fatalf("bot downloaded APK: %d", status)
		}
		status, data := request(t, server.URL+path, "GET", "", f.Token)
		if status != 200 || !bytes.Equal(data, apk) {
			t.Fatalf("authorized APK download: %d %q", status, data)
		}
	}
}
