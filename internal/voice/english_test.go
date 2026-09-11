package voice

import "testing"

func TestEnglishCommandsAndEndings(t *testing.T) {
	for _, test := range []struct {
		text, unit string
		q          float64
	}{
		{"two packs of grapes", "pakker", 2}, {"yes, 1 bag of apples", "poser", 1}, {"add three cans of tomatoes to the shopping list", "dåser", 3},
		{"a bottle of water", "flasker", 1}, {"two bunches of parsley", "bundter", 2}, {"two trays of mushrooms", "bakker", 2}, {"one crate of soda", "kasser", 1},
		{"two liters of milk", "liter", 2}, {"250 milliliters of cream", "milliliter", 250}, {"two kilos of potatoes", "kilo", 2}, {"500 grams of rice", "gram", 500}, {"two pieces of fruit", "stk.", 2},
		{"one and a half liters of milk", "liter", 1.5}, {"half a liter of milk", "liter", .5}, {"two point five liters of milk", "liter", 2.5},
	} {
		command, err := Parse(test.text)
		if err != nil || command.Quantity != test.q || command.Unit != test.unit {
			t.Errorf("%q: %+v %v", test.text, command, err)
		}
	}
	for _, text := range []string{"no thanks", "no thank you", "nothing else", "that's all", "goodbye", "cancel", "thank you", "nope"} {
		if !Ending(text) {
			t.Errorf("missed ending %q", text)
		}
	}
	for _, text := range []string{"yes", "yes please", "sure", "yes do it"} {
		if yes, clear := Confirmation(text); !yes || !clear {
			t.Errorf("missed confirmation %q", text)
		}
	}
	if _, clear := Confirmation("maybe"); clear {
		t.Fatal("ambiguous confirmation accepted")
	}
	c := Command{Text: "grapes", Quantity: 1, Unit: "pakker"}
	if c.ReplyLocalized("en") != "Added 1 pack of grapes to the shopping list." || c.ReplyLocalized("da") != "Tilføjet 1 pakke grapes til indkøbslisten." {
		t.Fatal("wrong reply language")
	}
}
