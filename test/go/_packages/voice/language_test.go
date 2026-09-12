package voice

import (
	"errors"
	"testing"

	"github.com/jonasrappy/foodie/internal/i18n"
)

func TestVocabularyIsSelectedNotMerged(t *testing.T) {
	for _, language := range []string{"en", "da"} {
		t.Run(language, func(t *testing.T) {
			p := New(language)
			other := "en"
			if language == "en" {
				other = "da"
			}
			own, foreign := i18n.Get(language).Voice, i18n.Get(other).Voice
			// Exercise every spoken unit alias, including attached numeric quantities.
			for unit, aliases := range own.UnitAliases {
				for _, alias := range aliases {
					for _, separator := range []string{" ", ""} {
						command, err := p.Parse("2" + separator + alias + " product")
						if err != nil || command.Quantity != 2 || command.Unit != unit || command.Text != "product" {
							t.Fatalf("%s: %+v %v", alias, command, err)
						}
					}
				}
			}
			for _, aliases := range foreign.UnitAliases {
				for _, alias := range aliases {
					if _, shared := p.units[alias]; shared {
						continue
					}
					if _, err := p.Parse("2" + alias + " product"); !errors.Is(err, ErrUnclear) {
						t.Fatalf("foreign attached unit %q was recognized", alias)
					}
					command, err := p.Parse("2 " + alias + " product")
					if err == nil && (command.Unit != "piece" || command.Text != alias+" product") {
						t.Fatalf("foreign unit was interpreted: %+v", command)
					}
				}
			}
			for word := range foreign.Numbers {
				if _, shared := own.Numbers[word]; shared {
					continue
				}
				if _, ok := p.integer(word); ok {
					t.Fatalf("foreign number %q recognized", word)
				}
			}
			for _, ending := range foreign.Endings {
				if contains(own.Endings, ending) {
					continue
				}
				if p.Ending(ending) {
					t.Fatalf("foreign ending %q recognized", ending)
				}
			}
			for _, reply := range append(foreign.Confirmations, foreign.Endings...) {
				if contains(own.Confirmations, reply) || contains(own.Endings, reply) {
					continue
				}
				if _, clear := p.Confirmation(reply); clear {
					t.Fatalf("foreign reply %q changed the conversation", reply)
				}
			}
			for _, reply := range own.Confirmations {
				if yes, clear := p.Confirmation(reply); !yes || !clear {
					t.Fatalf("own confirmation %q missed", reply)
				}
			}
			for _, reply := range own.Endings {
				if yes, clear := p.Confirmation(reply); yes || !clear {
					t.Fatalf("own ending %q missed", reply)
				}
			}
		})
	}
}

func TestConfiguredNumberGrammar(t *testing.T) {
	for _, test := range []struct {
		language, text string
		quantity       float64
	}{
		{"en", "twenty-six cans of tomatoes", 26}, {"en", "twenty six cans of tomatoes", 26}, {"en", "five hundred grams of rice", 500}, {"en", "one hundred and twenty cans of tomatoes", 120}, {"en", "two point five liters of milk", 2.5}, {"en", "one and a half liters of milk", 1.5}, {"en", "half a liter of milk", .5},
		{"da", "seksogtyve dåser tomater", 26}, {"da", "fem hundrede gram ris", 500}, {"da", "et hundrede og tyve dåser tomater", 120}, {"da", "to komma fem liter mælk", 2.5}, {"da", "halvanden liter mælk", 1.5}, {"da", "en halv liter mælk", .5},
	} {
		command, err := New(test.language).Parse(test.text)
		if err != nil || command.Quantity != test.quantity {
			t.Errorf("%s: %+v %v", test.text, command, err)
		}
	}
	command, err := Parse("two packs of grapes")
	if err != nil || command.Unit != "pack" || command.Reply() != "Added 2 packs of grapes to the shopping list." {
		t.Fatal("default parser/reply must be English", command, err)
	}
	if yes, clear := Confirmation("ja tak"); yes || clear {
		t.Fatal("default confirmation accepted Danish")
	}
}
