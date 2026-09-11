package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jonasrappy/foodie/internal/store"
	"net/http"
	"testing"
)

func sendVoiceText(t *testing.T, url, token, text, id, confirmation string) (int, store.VoiceResponse) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"text": text})
	request, _ := http.NewRequest("POST", url+"/api/voice/text", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", id)
	request.Header.Set("X-Voice-Confirmation", confirmation)
	result, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	var reply store.VoiceResponse
	if err := json.NewDecoder(result.Body).Decode(&reply); err != nil {
		t.Fatal(err)
	}
	return result.StatusCode, reply
}

func TestAndroidTextVoiceDialog(t *testing.T) {
	s, server, f := testServer(t)
	send := func(text, id, confirmation string) (int, store.VoiceResponse) {
		return sendVoiceText(t, server.URL, f.Token, text, id, confirmation)
	}
	if code, reply := send("tre stk vindruer", "android-add-00000001", ""); code != 200 || reply.Kind != "added" {
		t.Fatal(code, reply)
	}
	code, reply := send("vindruer", "android-add-00000002", "")
	if code != 200 || reply.Kind != "confirm" {
		t.Fatal(code, reply)
	}
	if code, reply = send("måske", "android-answer-00001", reply.ConfirmationID); code != 200 || reply.Kind != "confirm" {
		t.Fatal(code, reply)
	}
	if code, reply = send("ja tak", "android-answer-00002", "android-add-00000002"); code != 200 || reply.Speech != "Tilføjet 1 stk. vindruer til indkøbslisten." {
		t.Fatal(code, reply)
	}
	state, _ := s.store.Snapshot(t.Context())
	if len(state.Shopping) != 1 || state.Shopping[0].Quantity != 4 {
		t.Fatal(state)
	}
	if code, _ := send("ja tak", "android-answer-00002", "android-add-00000002"); code != 200 {
		t.Fatal(code)
	}
	again, _ := s.store.Snapshot(t.Context())
	if again.Revision != state.Revision {
		t.Fatal("replayed answer changed quantity")
	}
}

func TestAndroidVoiceFollowUps(t *testing.T) {
	s, server, f := testServer(t)
	send := func(text, id, confirmation string) (int, store.VoiceResponse) {
		return sendVoiceText(t, server.URL, f.Token, text, id, confirmation)
	}
	for i, text := range []string{"vindruer", "ja 1 pakke ostehaps", "ja, en liter mælk", "to poser kartofler", "en pakke smør", "tre dåser tomater"} {
		code, reply := send(text, fmt.Sprintf("follow-up-add-%08d", i), "")
		if code != 200 || reply.Kind != "added" {
			t.Fatal(text, code, reply)
		}
		if i == 1 && reply.Speech != "Tilføjet 1 pakke ostehaps til indkøbslisten." {
			t.Fatal(reply)
		}
	}
	state, err := s.store.Snapshot(t.Context())
	if err != nil || len(state.Shopping) != 6 {
		t.Fatal(state, err)
	}
	for i, text := range []string{"nej", "nej tak", "næ", "ellers tak", "det var det", "farvel", "Hej hej, Foodie!"} {
		code, reply := send(text, fmt.Sprintf("follow-up-end-%08d", i), "")
		if code != 200 || reply.Kind != "cancelled" || reply.Speech != "Ok" {
			t.Fatal(text, code, reply)
		}
	}
	for i, text := range []string{"ja", "jo", "ja tak", "gerne"} {
		code, reply := send(text, fmt.Sprintf("follow-up-yes-%08d", i), "")
		if code != 200 || reply.Kind != "retry" || reply.Speech != "Hvad skal jeg tilføje til indkøbslisten?" {
			t.Fatal(text, code, reply)
		}
	}
	code, question := send("ja 1 pakke ostehaps", "follow-up-duplicate-01", "")
	if code != 200 || question.Kind != "confirm" {
		t.Fatal(code, question)
	}
	if code, reply := send("Næ, tak.", "follow-up-decline-0001", question.ConfirmationID); code != 200 || reply.Kind != "cancelled" {
		t.Fatal(code, reply)
	}
	after, err := s.store.Snapshot(t.Context())
	if err != nil || after.Revision != state.Revision || len(after.Shopping) != 6 {
		t.Fatal("dialogue replies changed shopping data", after, err)
	}
}

func TestVoiceTextBoundary(t *testing.T) {
	s, server, f := testServer(t)
	for _, token := range []string{"", f.Bot} {
		expected := 401
		if token == f.Bot {
			expected = 403
		}
		code, _ := request(t, server.URL+"/api/voice/text", "POST", `{"text":"vindruer"}`, token)
		if code != expected {
			t.Fatal("voice authorization", code)
		}
	}
	for _, body := range []string{`{"text":"vindruer","quantity":9000}`, `{"text":null}`, `{"text":""}`, `{"text":"vindruer"} {}`} {
		code, _ := request(t, server.URL+"/api/voice/text", "POST", body, f.Token)
		if code != 400 {
			t.Fatal("invalid body", code, body)
		}
	}
	req, _ := http.NewRequest("POST", server.URL+"/api/voice/text", bytes.NewReader([]byte("RIFF")))
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "audio/wav")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 415 {
		t.Fatal("accepted audio", res.StatusCode)
	}
	state, _ := s.store.Snapshot(t.Context())
	if len(state.Shopping) != 0 || state.Revision != 0 {
		t.Fatal("invalid request changed data")
	}
}
