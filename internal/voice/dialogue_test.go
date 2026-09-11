package voice

import (
	"errors"
	"testing"
)

func TestEndConversation(t *testing.T) {
	for _, said := range []string{
		"nej", "Nej tak!", "NÆ", "næh", "Næ, tak.", "nix", "niks", "nope",
		"ellers tak", "Nej, ellers tak.", "nej, det var det", "nej tak, det var det",
		"ikke mere", "ikke andet", "ingenting", "det var det hele", "det var alt for nu",
		"det er nok", "jeg er færdig", "stop", "annullér", "lad være",
		"Farvel!", "hej hej", "vi ses", "tak for hjælpen", "bye bye",
		"Nej tak, Foodie.", "Det var det, Foody!", "  Næ,   tak!  ",
	} {
		t.Run(said, func(t *testing.T) {
			if _, err := Parse(said); !errors.Is(err, ErrCancelled) || err.Error() != "Ok" {
				t.Fatalf("ending became a grocery or wrong reply: %v", err)
			}
			if yes, clear := Confirmation(said); yes || !clear {
				t.Fatal("ending must also decline a duplicate addition")
			}
		})
	}
}

func TestAffirmativeNeedsAnItem(t *testing.T) {
	for _, said := range []string{"ja", "Ja tak!", "jo", "gerne", "ja gerne", "jep", "jeps", "yes", "Ja, Foodie!"} {
		if _, err := Parse(said); !errors.Is(err, ErrNeedsItem) {
			t.Fatalf("%q must ask which item, got %v", said, err)
		}
	}
	if yes, clear := Confirmation("Ja, tak, Foodie!"); !yes || !clear {
		t.Fatal("a positive answer to a duplicate question must still confirm it")
	}
}

func TestAffirmativeWithAnItem(t *testing.T) {
	for _, tc := range []struct {
		said string
		want Command
	}{
		{"ja 1 pakke ostehaps", Command{"ostehaps", 1, "pakker"}},
		{"Ja, en pakke ostehaps.", Command{"ostehaps", 1, "pakker"}},
		{"ja, en liter mælk", Command{"mælk", 1, "liter"}},
		{"jo to poser kartofler", Command{"kartofler", 2, "poser"}},
		{"ja tilføj vindruer", Command{"vindruer", 1, "stk."}},
		{"ja skriv en pakke smør på listen", Command{"smør", 1, "pakker"}},
		{"ja halvanden liter mælk", Command{"mælk", 1.5, "liter"}},
		{"jo, seksogtyve dåser tomater", Command{"tomater", 26, "dåser"}},
	} {
		got, err := Parse(tc.said)
		if err != nil || got != tc.want {
			t.Fatalf("%q: got %+v, %v; want %+v", tc.said, got, err, tc.want)
		}
	}
	got, _ := Parse("ja 1 pakke ostehaps")
	if reply := got.Reply(); reply != "Tilføjet 1 pakke ostehaps til indkøbslisten." {
		t.Fatal(reply)
	}
}

func TestConversationWordsAreNotSubstringMatches(t *testing.T) {
	for _, said := range []string{"nejliker", "næsepuder", "farvel til kalk", "ja tak kaffe", "stop madspild brød"} {
		got, err := Parse(said)
		if err != nil || got.Text != said || got.Quantity != 1 || got.Unit != "stk." {
			t.Fatalf("product %q mistaken for a reply: %+v, %v", said, got, err)
		}
		if _, clear := Confirmation(said); clear {
			t.Fatalf("product %q must not approve or decline a duplicate", said)
		}
	}
}
