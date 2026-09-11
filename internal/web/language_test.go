package web

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestEnglishUIBotExportAndVoiceConversation(t *testing.T) {
	s, server, f := testServer(t)
	s.language = "en"
	s.timezone = "UTC"
	status, html := request(t, server.URL+"/", "GET", "", "")
	if status != 200 || !strings.Contains(string(html), `lang="en"`) || !strings.Contains(string(html), "Shopping list") || strings.Contains(string(html), ">Indkøbsliste<") {
		t.Fatal("English HTML not rendered")
	}
	status, pack := request(t, server.URL+"/language.js", "GET", "", "")
	if status != 200 || !strings.Contains(string(pack), `"locale":"en-US"`) {
		t.Fatal("missing browser language pack")
	}
	for index, phrase := range []string{"two packs of grapes", "three bags of potatoes", "one can of tomatoes", "two bottles of water", "one bunch of parsley", "two trays of mushrooms", "one crate of soda", "two liters of milk", "100 milliliters of cream", "one kilo of carrots", "500 grams of rice", "two pieces of fruit"} {
		code, reply := sendVoiceText(t, server.URL, f.Token, phrase, fmt.Sprintf("english-add-%08d", index), "")
		if code != 200 || reply.Kind != "added" || !strings.HasPrefix(reply.Speech, "Added ") {
			t.Fatalf("English add: %d %+v", code, reply)
		}
	}
	code, reply := sendVoiceText(t, server.URL, f.Token, "one pack of grapes", "english-duplicate-00001", "")
	if code != 200 || reply.Kind != "confirm" || !strings.Contains(reply.Speech, "2 packs of grapes") || !strings.Contains(reply.Speech, "3 packs?") {
		t.Fatal(reply)
	}
	code, reply = sendVoiceText(t, server.URL, f.Token, "yes please", "english-confirm-000001", reply.ConfirmationID)
	if code != 200 || reply.Speech != "Added 1 pack of grapes to the shopping list." {
		t.Fatal(reply)
	}
	_, data := request(t, server.URL+"/api/v1/requirements", "GET", "", f.Bot)
	var result requirementsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Timezone != "UTC" || !strings.Contains(result.Instructions, "Do not buy") {
		t.Fatal("bot metadata is not localized")
	}
	found := false
	for _, line := range result.RequiredShoppingItems {
		if line == "3 packs grapes" {
			found = true
		}
	}
	if !found {
		t.Fatal("English unit labels missing from bot export")
	}
	code, reply = sendVoiceText(t, server.URL, f.Token, "no thanks", "english-end-00000001", "")
	if code != 200 || reply.Kind != "cancelled" {
		t.Fatal(reply)
	}
}
