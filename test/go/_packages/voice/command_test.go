package voice

import (
	"errors"
	"testing"
)

func TestDanishShoppingCommands(t *testing.T) {
	for _, tc := range []struct {
		said, text string
		q          float64
		unit       string
	}{
		{"1 pakke vindruer", "vindruer", 1, "pack"}, {"en pakke vindruer", "vindruer", 1, "pack"},
		{"2 pakker Pepsi Max", "pepsi max", 2, "pack"}, {"to pk. Pepsi Max", "pepsi max", 2, "pack"},
		{"1 kasse pepsi max", "pepsi max", 1, "crate"}, {"en kasse Pepsi Max", "pepsi max", 1, "crate"},
		{"to kasser Pepsi Max", "pepsi max", 2, "crate"}, {"ja 2kasser Pepsi Max", "pepsi max", 2, "crate"},
		{"2pakker Pepsi Max", "pepsi max", 2, "pack"}, {"ja, 2pk. Pepsi Max", "pepsi max", 2, "pack"},
		{"250millilitre fløde", "fløde", 250, "milliliter"}, {"1,5l. mælk", "mælk", 1.5, "liter"},
		{"2poser kartofler", "kartofler", 2, "bag"}, {"2dåser tomater", "tomater", 2, "can"},
		{"2fl. vand", "vand", 2, "bottle"}, {"1bdt. persille", "persille", 1, "bunch"},
		{"2bk. vindruer", "vindruer", 2, "tray"}, {"2stk. æg", "æg", 2, "piece"},
		{"5kg. kartofler", "kartofler", 5, "kilogram"}, {"500g. mel", "mel", 500, "gram"},
		{"pepsi max 2pakker", "pepsi max 2pakker", 1, "piece"},
		{"tilføj vindruer til indkøbslisten tak", "vindruer", 1, "piece"}, {"500g pasta", "pasta", 500, "gram"}, {"halvandet kilo æbler", "æbler", 1.5, "kilogram"}, {"vindruer", "vindruer", 1, "piece"}, {"Tilføj vindruer til indkøbslisten.", "vindruer", 1, "piece"}, {"to bakker vindruer", "vindruer", 2, "tray"}, {"1 stk vindruer", "vindruer", 1, "piece"}, {"en pose kartofler", "kartofler", 1, "bag"}, {"fem hundrede gram pasta", "pasta", 500, "gram"}, {"500 gram pasta", "pasta", 500, "gram"}, {"to og en halv liter mælk", "mælk", 2.5, "liter"}, {"en halv liter fløde", "fløde", .5, "liter"}, {"1,5 kilo æbler", "æbler", 1.5, "kilogram"}, {"en komma fem kilo æbler", "æbler", 1.5, "kilogram"}, {"seksogtyve dåser tomater", "tomater", 26, "can"}, {"tre flasker vand på indkøbslisten", "vand", 3, "bottle"}, {"salt og peber", "salt og peber", 1, "piece"}, {"Hej foodie tilføj to liter mælk", "mælk", 2, "liter"},
	} {
		t.Run(tc.said, func(t *testing.T) {
			got, err := New("da").Parse(tc.said)
			if err != nil || got.Text != tc.text || got.Quantity != tc.q || got.Unit != tc.unit {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
	c, _ := New("da").Parse("vindruer")
	if c.ReplyLocalized("da") != "Tilføjet 1 stk. vindruer til indkøbslisten." {
		t.Fatal(c.ReplyLocalized("da"))
	}
	c, _ = New("da").Parse("1 kasse pepsi max")
	if c.ReplyLocalized("da") != "Tilføjet 1 kasse pepsi max til indkøbslisten." {
		t.Fatal(c.ReplyLocalized("da"))
	}
}
func TestUnclearAndCancelled(t *testing.T) {
	for _, s := range []string{"", "1", "nul æbler", "10000 gram mel", "-1 æble", "1,234 liter mælk", "2ukendt smør", "slet indkøbslisten", "tak fordi du så med", "[musik]", "hvad skal jeg tilføje", "tilføj"} {
		if c, err := New("da").Parse(s); err == nil {
			t.Fatalf("accepted %q: %+v", s, c)
		}
	}
	for _, s := range []string{"Stop", "nej tak", "annuller", "ingenting"} {
		if _, err := New("da").Parse(s); !errors.Is(err, ErrCancelled) {
			t.Fatal(s, err)
		}
	}
}
