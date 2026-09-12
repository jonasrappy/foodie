package web

import (
	"encoding/json"
	"github.com/jonasrappy/foodie/internal/i18n"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Keep speech, persisted amounts and Grok's requirements in step with both
// frontend dropdowns. A newly offered unit must also work through this API.
func TestVoiceUnderstandsEveryDropdownUnit(t *testing.T) {
	cases := []struct{ unit, one, many, name string }{
		{"piece", "et stykke", "to stykker", "agurk"},
		{"liter", "en liter", "to litre", "mælk"},
		{"milliliter", "en milliliter", "to millilitre", "fløde"},
		{"kilogram", "et kilo", "to kilogram", "kartofler"},
		{"gram", "et gram", "to gram", "sukker"},
		{"pack", "en pakke", "2 pakker", "pepsi max"},
		{"bag", "en pose", "to poser", "ris"},
		{"can", "en dåse", "to dåser", "tomater"},
		{"bottle", "en flaske", "to flasker", "vand"},
		{"bunch", "et bundt", "to bundter", "persille"},
		{"tray", "en bakke", "to bakker", "vindruer"},
		{"crate", "en kasse", "to kasser", "pepsi max"},
	}
	html, err := os.ReadFile(filepath.Join(testPublicDir(), "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	covered := make(map[string]bool)
	for _, tc := range cases {
		covered[tc.unit] = true
	}
	for _, id := range []string{"shopping-unit", "edit-unit"} {
		selectHTML := regexp.MustCompile(`(?s)<select id="` + id + `"[^>]*>(.*?)</select>`).FindSubmatch(html)
		if len(selectHTML) != 2 {
			t.Fatalf("missing %s dropdown", id)
		}
		options := regexp.MustCompile(`<option value="([^"]+)"[^>]*>[^<]+</option>`).FindAllSubmatch(selectHTML[1], -1)
		if len(options) != len(cases) {
			t.Fatalf("voice coverage does not match %s: %d dropdown units, %d tested units", id, len(options), len(cases))
		}
		for _, option := range options {
			if !covered[strings.TrimSpace(string(option[1]))] {
				t.Fatalf("dropdown unit %q lacks speech coverage", option[1])
			}
		}
	}
	for _, tc := range cases {
		t.Run(tc.unit, func(t *testing.T) {
			s, server, f := testServer(t)
			code, reply := sendVoiceText(t, server.URL, f.Token, tc.many+" "+tc.name, "unit-add-00000000001", "")
			if code != 200 || reply.Kind != "added" || reply.Speech != "Tilføjet 2 "+i18n.Unit("da", tc.unit, 2)+" "+tc.name+" til indkøbslisten." {
				t.Fatal(code, reply)
			}
			state, err := s.store.Snapshot(t.Context())
			if err != nil || len(state.Shopping) != 1 {
				t.Fatal(state, err)
			}
			item := state.Shopping[0]
			if item.Text != tc.name || item.Quantity != 2 || item.Unit != tc.unit {
				t.Fatal("spoken unit was not preserved", item)
			}
			code, data := request(t, server.URL+"/api/v1/requirements", "GET", "", f.Bot)
			var requirements struct {
				Shopping []string `json:"required_shopping_items"`
			}
			if code != 200 || json.Unmarshal(data, &requirements) != nil || len(requirements.Shopping) != 1 || requirements.Shopping[0] != "2 "+i18n.Unit("da", tc.unit, 2)+" "+tc.name {
				t.Fatalf("Grok received the wrong amount: %d %s", code, data)
			}
			// Singular speech must find the existing unit and ask before increasing it.
			code, reply = sendVoiceText(t, server.URL, f.Token, tc.one+" "+tc.name, "unit-duplicate-00001", "")
			if code != 200 || reply.Kind != "confirm" || !strings.Contains(reply.Speech, "bliver 3 "+i18n.Unit("da", tc.unit, 3)) {
				t.Fatal("singular unit failed to match plural", code, reply)
			}
			code, reply = sendVoiceText(t, server.URL, f.Token, "ja tak", "unit-answer-00000001", reply.ConfirmationID)
			if code != 200 || reply.Kind != "added" {
				t.Fatal(code, reply)
			}
			state, err = s.store.Snapshot(t.Context())
			if err != nil || len(state.Shopping) != 1 || state.Shopping[0].Quantity != 3 || state.Shopping[0].Unit != tc.unit {
				t.Fatal("duplicate confirmation changed the unit", state, err)
			}
		})
	}
}
