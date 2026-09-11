// Package voice handles a single, explicitly requested shopping addition.
package voice

import (
	"errors"
	"fmt"
	"github.com/jonasrappy/foodie/internal/i18n"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type Command struct {
	Text     string  `json:"text"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

var ErrUnclear = errors.New("Jeg forstod ikke varen. Sig for eksempel to bakker vindruer.")
var ErrCancelled = errors.New("Ok")
var ErrNeedsItem = errors.New("Hvad skal jeg tilføje til indkøbslisten?")
var attachedUnit = regexp.MustCompile(`^(\d+(?:[,.]\d{1,2})?)([\p{L}]+\.?)(\s|$)`)
var decimal = regexp.MustCompile(`^\d+(?:[,.]\d{1,2})?$`)
var units = map[string]string{
	"piece": "stk.", "pieces": "stk.", "item": "stk.", "items": "stk.", "pc": "stk.", "pcs": "stk.",
	"liters": "liter", "litres": "liter", "milliliters": "milliliter", "millilitres": "milliliter", "grams": "gram", "kilos": "kilo", "kilograms": "kilo",
	"pack": "pakker", "packs": "pakker", "packet": "pakker", "packets": "pakker", "package": "pakker", "packages": "pakker", "bag": "poser", "bags": "poser", "can": "dåser", "cans": "dåser", "bottle": "flasker", "bottles": "flasker", "bunch": "bundter", "bunches": "bundter", "tray": "bakker", "trays": "bakker", "crate": "kasser", "crates": "kasser", "case": "kasser", "cases": "kasser", "box": "kasser", "boxes": "kasser",

	"stk": "stk.", "stk.": "stk.", "styk": "stk.", "stykker": "stk.", "stykke": "stk.", "stykkerne": "stk.",
	"liter": "liter", "litre": "liter", "l": "liter", "milliliter": "milliliter", "millilitre": "milliliter", "mililiter": "milliliter", "mililitre": "milliliter", "ml": "milliliter",
	"gram": "gram", "g": "gram", "kilo": "kilo", "kilogram": "kilo", "kg": "kilo",
	"pakke": "pakker", "pakker": "pakker", "pk": "pakker", "pkt": "pakker", "pose": "poser", "poser": "poser", "dåse": "dåser", "dåser": "dåser", "ds": "dåser",
	"flaske": "flasker", "flasker": "flasker", "fl": "flasker", "bundt": "bundter", "bundter": "bundter", "bdt": "bundter", "bakke": "bakker", "bakker": "bakker", "bk": "bakker",
	"kasse": "kasser", "kasser": "kasser",
}

// Android may transcribe amounts as "500ml" or "2pk.". Split only a
// recognized unit at the beginning; never alter numbers inside a product name.
func separateAttachedUnit(text string) string {
	parts := attachedUnit.FindStringSubmatch(text)
	if parts != nil {
		if _, ok := units[strings.TrimSuffix(parts[2], ".")]; ok {
			return parts[1] + " " + text[len(parts[1]):]
		}
	}
	return text
}

var numbers = map[string]int{"a": 1, "an": 1, "zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90, "thousand": 1000, "nul": 0, "en": 1, "et": 1, "én": 1, "ét": 1, "to": 2, "tre": 3, "fire": 4, "fem": 5, "seks": 6, "syv": 7, "otte": 8, "ni": 9, "ti": 10, "elleve": 11, "tolv": 12, "tretten": 13, "fjorten": 14, "femten": 15, "seksten": 16, "sytten": 17, "atten": 18, "nitten": 19, "tyve": 20, "tredive": 30, "fyrre": 40, "halvtreds": 50, "tres": 60, "halvfjerds": 70, "firs": 80, "halvfems": 90, "hundrede": 100, "hundred": 100, "tusind": 1000}

func integer(s string) (int, bool) {
	if n, ok := numbers[s]; ok {
		return n, true
	}
	if pieces := strings.Split(s, "-"); len(pieces) == 2 {
		a, aok := numbers[pieces[0]]
		b, bok := numbers[pieces[1]]
		if aok && bok && a >= 20 && a <= 90 && b >= 1 && b <= 9 {
			return a + b, true
		}
	}
	parts := strings.Split(s, "og")
	if len(parts) == 2 {
		a, aok := numbers[parts[0]]
		b, bok := numbers[parts[1]]
		if aok && bok && a >= 1 && a <= 9 && b >= 20 && b <= 90 {
			return a + b, true
		}
	}
	return 0, false
}
func amount(words []string) (float64, int, error) {
	if len(words) == 0 {
		return 1, 0, nil
	}
	if words[0] == "halvanden" || words[0] == "halvandet" {
		return 1.5, 1, nil
	}
	if words[0] == "halv" || words[0] == "halvt" || words[0] == "half" {
		if len(words) > 1 && (words[1] == "a" || words[1] == "an") {
			return .5, 2, nil
		}
		return .5, 1, nil
	}
	var value float64
	used := 1
	if decimal.MatchString(words[0]) {
		value, _ = strconv.ParseFloat(strings.ReplaceAll(words[0], ",", "."), 64)
	} else if n, ok := integer(words[0]); ok {
		value = float64(n)
	} else {
		if len(words[0]) > 0 && (unicode.IsDigit([]rune(words[0])[0]) || words[0] == "minus" || strings.HasPrefix(words[0], "-")) {
			return 0, 0, ErrUnclear
		}
		return 1, 0, nil
	}
	if len(words) > 1 && (words[1] == "hundrede" || words[1] == "hundred" || words[1] == "tusind" || words[1] == "thousand") {
		mult, _ := integer(words[1])
		value *= float64(mult)
		used++
		if len(words) > used && (words[used] == "og" || words[used] == "and") {
			if len(words) > used+1 {
				if n, ok := integer(words[used+1]); ok {
					value += float64(n)
					used += 2
				}
			}
		}
	}
	if len(words) > used && (words[used] == "komma" || words[used] == "point") {
		if len(words) <= used+1 {
			return 0, 0, ErrUnclear
		}
		fraction, fok := integer(words[used+1])
		if !fok || fraction > 99 {
			return 0, 0, ErrUnclear
		}
		div := 10.0
		if fraction >= 10 {
			div = 100
		}
		value += float64(fraction) / div
		used += 2
	}
	if value >= 20 && value <= 90 && int(value)%10 == 0 && len(words) > used {
		if n, ok := integer(words[used]); ok && n > 0 && n < 10 {
			value += float64(n)
			used++
		}
	}
	if value == 1 && len(words) > used && (words[used] == "halv" || words[used] == "halvt" || words[used] == "half") {
		value = .5
		used++
	} else if len(words) > used+2 && (words[used] == "og" || words[used] == "and") && (words[used+1] == "en" || words[used+1] == "et" || words[used+1] == "a") && (words[used+2] == "halv" || words[used+2] == "halvt" || words[used+2] == "half") {
		value += .5
		used += 3
	}
	if value < .01 || value > 9999 || math.Abs(value*100-math.Round(value*100)) > 1e-6 {
		return 0, 0, ErrUnclear
	}
	return value, used, nil
}
func Parse(transcript string) (Command, error) {
	text := strings.ToLower(strings.TrimSpace(transcript))
	text = strings.Trim(text, " .!?,;:\"“”")
	if len([]rune(text)) > 300 || strings.ContainsAny(text, "\n\r[]{}<>") {
		return Command{}, ErrUnclear
	}
	if Ending(text) {
		return Command{}, ErrCancelled
	}
	if wantsAnother(text) {
		return Command{}, ErrNeedsItem
	}
	for _, bad := range []string{"undertekster", "tak fordi", "tak for at", "subscribe", "abonner", "musik", "www.", "http"} {
		if strings.Contains(text, bad) {
			return Command{}, ErrUnclear
		}
	}
	text = additionPrefix(text)
	for _, prefix := range []string{"hey foodie ", "hej foodie ", "hey foody "} {
		text = strings.TrimPrefix(text, prefix)
	}
	for _, prefix := range []string{"please add ", "can you add ", "could you add ", "add ", "put ", "i would like ", "i need ", "jeg vil gerne have ", "vil du tilføje ", "kan du tilføje ", "tilføj ", "tilføje ", "skriv ", "sæt "} {
		if strings.HasPrefix(text, prefix) {
			text = strings.TrimPrefix(text, prefix)
			break
		}
	}
	for {
		previous := text
		for _, suffix := range []string{" to the shopping list", " on the shopping list", " to my shopping list", " to the list", " please", " til indkøbslisten", " på indkøbslisten", " til listen", " på listen", " tak"} {
			text = strings.TrimSuffix(text, suffix)
		}
		if text == previous {
			break
		}
	}
	text = separateAttachedUnit(text)
	words := strings.Fields(text)
	for i := range words {
		words[i] = strings.Trim(words[i], ",;:")
	}
	quantity, n, err := amount(words)
	if err != nil {
		return Command{}, err
	}
	words = words[n:]
	unit := "stk."
	if len(words) > 0 {
		if u, ok := units[strings.TrimSuffix(words[0], ".")]; ok {
			unit = u
			words = words[1:]
		}
	}
	if len(words) > 0 && (words[0] == "af" || words[0] == "of") {
		words = words[1:]
	}
	name := strings.Join(words, " ")
	hasLetter := false
	for _, r := range name {
		hasLetter = hasLetter || unicode.IsLetter(r)
	}
	if name == "add" || name == "put" || name == "tilføj" || name == "tilføje" || name == "skriv" || name == "sæt" || !hasLetter || len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		return Command{}, ErrUnclear
	}
	for _, bad := range []string{"delete ", "remove ", "reset ", "buy ", "order ", "add ", "what ", "i ", "you ", "slet ", "fjern ", "nulstil ", "køb ", "bestil ", "tilføj ", "jeg ", "hvad ", "du "} {
		if strings.HasPrefix(name, bad) {
			return Command{}, ErrUnclear
		}
	}
	return Command{Text: name, Quantity: quantity, Unit: unit}, nil
}
func (c Command) Reply() string { return c.ReplyLocalized("da") }
func (c Command) ReplyLocalized(language string) string {
	return fmt.Sprintf(i18n.Text(language, "Tilføjet %s %s %s til indkøbslisten."), i18n.Number(language, c.Quantity), i18n.Unit(language, c.Unit, c.Quantity), c.Text)
}

// Confirmation accepts explicit Danish yes/no only; ambiguous speech never edits a list.
func Confirmation(text string) (bool, bool) {
	if Ending(text) {
		return false, true
	}
	text = dialogueText(text)
	switch text {
	case "yes please", "yes do it", "yes add it", "sure", "please do", "ja", "ja tak", "ja gør det", "ja tilføj", "gerne", "det må du gerne", "yes":
		return true, true
	default:
		return false, false
	}
}
