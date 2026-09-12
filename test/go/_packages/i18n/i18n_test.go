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
		if translated, ok := da.Messages[key]; !ok || text == "" || text != key || strings.Count(key, "%s") != strings.Count(translated, "%s") {
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
	if Unit("en", "pack", 1) != "pack" || Unit("en", "pack", 2) != "packs" || Unit("da", "pack", 1) != "pakke" {
		t.Fatal("wrong unit forms")
	}
	source := `<html lang="en"><button aria-label="Add item">Add</button><option value="pack">packs</option></html>`
	translated := HTML("da", source)
	if !strings.Contains(translated, `lang="da"`) || !strings.Contains(translated, `aria-label="Tilføj vare"`) || !strings.Contains(translated, `value="pack"`) {
		t.Fatal("template localization altered IDs or missed labels")
	}
	if HTML("en", source) != source {
		t.Fatal("English template changed")
	}
	if Get("").Language != "en" {
		t.Fatal("English must be the default")
	}
	if fmt.Sprintf(Text("en", "Added %s %s of %s to the shopping list."), "2", "packs", "grapes") != "Added 2 packs of grapes to the shopping list." {
		t.Fatal("wrong speech format")
	}
}
