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
		{"1 pakke vindruer", "vindruer", 1, "pakker"}, {"en pakke vindruer", "vindruer", 1, "pakker"},
		{"2 pakker Pepsi Max", "pepsi max", 2, "pakker"}, {"to pk. Pepsi Max", "pepsi max", 2, "pakker"},
		{"1 kasse pepsi max", "pepsi max", 1, "kasser"}, {"en kasse Pepsi Max", "pepsi max", 1, "kasser"},
		{"to kasser Pepsi Max", "pepsi max", 2, "kasser"}, {"ja 2kasser Pepsi Max", "pepsi max", 2, "kasser"},
		{"2pakker Pepsi Max", "pepsi max", 2, "pakker"}, {"ja, 2pk. Pepsi Max", "pepsi max", 2, "pakker"},
		{"250millilitre fløde", "fløde", 250, "milliliter"}, {"1,5l. mælk", "mælk", 1.5, "liter"},
		{"2poser kartofler", "kartofler", 2, "poser"}, {"2dåser tomater", "tomater", 2, "dåser"},
		{"2fl. vand", "vand", 2, "flasker"}, {"1bdt. persille", "persille", 1, "bundter"},
		{"2bk. vindruer", "vindruer", 2, "bakker"}, {"2stk. æg", "æg", 2, "stk."},
		{"5kg. kartofler", "kartofler", 5, "kilo"}, {"500g. mel", "mel", 500, "gram"},
		{"pepsi max 2pakker", "pepsi max 2pakker", 1, "stk."},
		{"tilføj vindruer til indkøbslisten tak", "vindruer", 1, "stk."}, {"500g pasta", "pasta", 500, "gram"}, {"halvandet kilo æbler", "æbler", 1.5, "kilo"}, {"vindruer", "vindruer", 1, "stk."}, {"Tilføj vindruer til indkøbslisten.", "vindruer", 1, "stk."}, {"to bakker vindruer", "vindruer", 2, "bakker"}, {"1 stk vindruer", "vindruer", 1, "stk."}, {"en pose kartofler", "kartofler", 1, "poser"}, {"fem hundrede gram pasta", "pasta", 500, "gram"}, {"500 gram pasta", "pasta", 500, "gram"}, {"to og en halv liter mælk", "mælk", 2.5, "liter"}, {"en halv liter fløde", "fløde", .5, "liter"}, {"1,5 kilo æbler", "æbler", 1.5, "kilo"}, {"en komma fem kilo æbler", "æbler", 1.5, "kilo"}, {"seksogtyve dåser tomater", "tomater", 26, "dåser"}, {"tre flasker vand på indkøbslisten", "vand", 3, "flasker"}, {"salt og peber", "salt og peber", 1, "stk."}, {"Hej foodie tilføj to liter mælk", "mælk", 2, "liter"},
	} {
		t.Run(tc.said, func(t *testing.T) {
			got, err := Parse(tc.said)
			if err != nil || got.Text != tc.text || got.Quantity != tc.q || got.Unit != tc.unit {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
	c, _ := Parse("vindruer")
	if c.Reply() != "Tilføjet 1 stk. vindruer til indkøbslisten." {
		t.Fatal(c.Reply())
	}
	c, _ = Parse("1 kasse pepsi max")
	if c.Reply() != "Tilføjet 1 kasse pepsi max til indkøbslisten." {
		t.Fatal(c.Reply())
	}
}
func TestUnclearAndCancelled(t *testing.T) {
	for _, s := range []string{"", "1", "nul æbler", "10000 gram mel", "-1 æble", "1,234 liter mælk", "2ukendt smør", "slet indkøbslisten", "tak fordi du så med", "[musik]", "hvad skal jeg tilføje", "tilføj"} {
		if c, err := Parse(s); err == nil {
			t.Fatalf("accepted %q: %+v", s, c)
		}
	}
	for _, s := range []string{"Stop", "nej tak", "annuller", "ingenting"} {
		if _, err := Parse(s); !errors.Is(err, ErrCancelled) {
			t.Fatal(s, err)
		}
	}
}
