package i18n

import (
	"fmt"
	"strings"
	"testing"
)

func TestPackParityAndSafeTemplates(t *testing.T) {
	en, da := Get("en"), Get("da")
	if len(en.Messages) != len(da.Messages) || len(en.Units) != 12 || len(da.Units) != 12 {
		t.Fatal("language packs differ")
	}
	for key, text := range en.Messages {
		if _, ok := da.Messages[key]; !ok || text == "" {
			t.Fatalf("missing translation: %q", key)
		}
		if strings.Count(key, "%s") != strings.Count(text, "%s") {
			t.Fatalf("format placeholders differ for %q", key)
		}
	}
	for unit := range en.Units {
		if _, ok := da.Units[unit]; !ok {
			t.Fatal("unit missing")
		}
	}
	if Unit("en", "pakker", 1) != "pack" || Unit("en", "pakker", 2) != "packs" || Unit("da", "pakker", 1) != "pakke" {
		t.Fatal("wrong unit forms")
	}
	source := `<html lang="da"><button aria-label="Tilføj vare">Tilføj</button><option value="pakker">pakker</option></html>`
	translated := HTML("en", source)
	if !strings.Contains(translated, `lang="en"`) || !strings.Contains(translated, `aria-label="Add item"`) || !strings.Contains(translated, `value="pakker"`) {
		t.Fatal("template localization altered IDs or missed labels")
	}
	if HTML("da", source) != source {
		t.Fatal("Danish template changed")
	}
	if Get("").Language != "en" {
		t.Fatal("English must be the default")
	}
	if fmt.Sprintf(Text("en", "Tilføjet %s %s %s til indkøbslisten."), "2", "packs", "grapes") != "Added 2 packs of grapes to the shopping list." {
		t.Fatal("wrong speech format")
	}
}
