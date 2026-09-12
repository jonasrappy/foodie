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

var ErrUnclear = errors.New("I didn't understand the item. Try saying two packs of grapes.")
var ErrCancelled = errors.New("Ok")
var ErrNeedsItem = errors.New("What should I add to the shopping list?")
var attachedUnit = regexp.MustCompile(`^(\d+(?:[,.]\d{1,2})?)([\p{L}]+\.?)(\s|$)`)
var decimal = regexp.MustCompile(`^\d+(?:[,.]\d{1,2})?$`)

// Parser owns the vocabulary of one configured language.
type Parser struct {
	grammar i18n.VoiceGrammar
	units   map[string]string
}

func New(language string) *Parser {
	grammar := i18n.Get(language).Voice
	parser := &Parser{grammar: grammar, units: make(map[string]string)}
	for unit, aliases := range grammar.UnitAliases {
		for _, alias := range aliases {
			parser.units[alias] = unit
		}
	}
	return parser
}

// Android may transcribe amounts as "500ml" or "2pk.". Split only a
// recognized unit at the beginning; never alter numbers inside a product name.
func (p *Parser) separateAttachedUnit(text string) string {
	parts := attachedUnit.FindStringSubmatch(text)
	if parts != nil {
		if _, ok := p.units[strings.TrimSuffix(parts[2], ".")]; ok {
			return parts[1] + " " + text[len(parts[1]):]
		}
	}
	return text
}

func (p *Parser) integer(s string) (int, bool) {
	if n, ok := p.grammar.Numbers[s]; ok {
		return n, true
	}
	parts := strings.Split(s, p.grammar.CompoundSeparator)
	if len(parts) == 2 {
		a, aok := p.grammar.Numbers[parts[0]]
		b, bok := p.grammar.Numbers[parts[1]]
		if p.grammar.CompoundReversed {
			a, b = b, a
		}
		if aok && bok && a >= 20 && a <= 90 && a%10 == 0 && b >= 1 && b <= 9 {
			return a + b, true
		}
	}
	return 0, false
}
func (p *Parser) amount(words []string) (float64, int, error) {
	if len(words) == 0 {
		return 1, 0, nil
	}
	if fraction, ok := p.grammar.Fractions[words[0]]; ok {
		if fraction == .5 && len(words) > 1 && contains(p.grammar.Articles, words[1]) {
			return fraction, 2, nil
		}
		return fraction, 1, nil
	}
	var value float64
	used := 1
	if decimal.MatchString(words[0]) {
		value, _ = strconv.ParseFloat(strings.ReplaceAll(words[0], ",", "."), 64)
	} else if n, ok := p.integer(words[0]); ok {
		value = float64(n)
	} else {
		if len(words[0]) > 0 && (unicode.IsDigit([]rune(words[0])[0]) || words[0] == p.grammar.NegativeWord || strings.HasPrefix(words[0], "-")) {
			return 0, 0, ErrUnclear
		}
		return 1, 0, nil
	}
	if len(words) > 1 && p.grammar.Multipliers[words[1]] > 0 {
		mult := p.grammar.Multipliers[words[1]]
		value *= float64(mult)
		used++
		if len(words) > used && words[used] == p.grammar.Conjunction {
			if len(words) > used+1 {
				if n, ok := p.integer(words[used+1]); ok {
					value += float64(n)
					used += 2
				}
			}
		}
	}
	if len(words) > used && words[used] == p.grammar.DecimalWord {
		if len(words) <= used+1 {
			return 0, 0, ErrUnclear
		}
		fraction, fok := p.integer(words[used+1])
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
	if p.grammar.SpacedTens && value >= 20 && value <= 90 && int(value)%10 == 0 && len(words) > used {
		if n, ok := p.integer(words[used]); ok && n > 0 && n < 10 {
			value += float64(n)
			used++
		}
	}
	if value == 1 && len(words) > used && p.grammar.Fractions[words[used]] == .5 {
		value = .5
		used++
	} else if len(words) > used+2 && words[used] == p.grammar.Conjunction && contains(p.grammar.Articles, words[used+1]) && p.grammar.Fractions[words[used+2]] == .5 {
		value += .5
		used += 3
	}
	if value < .01 || value > 9999 || math.Abs(value*100-math.Round(value*100)) > 1e-6 {
		return 0, 0, ErrUnclear
	}
	return value, used, nil
}
func Parse(transcript string) (Command, error) { return New("en").Parse(transcript) }

func (p *Parser) Parse(transcript string) (Command, error) {
	text := strings.ToLower(strings.TrimSpace(transcript))
	text = strings.Trim(text, " .!?,;:\"“”")
	if len([]rune(text)) > 300 || strings.ContainsAny(text, "\n\r[]{}<>") {
		return Command{}, ErrUnclear
	}
	if p.Ending(text) {
		return Command{}, ErrCancelled
	}
	if contains(p.grammar.Affirmatives, p.dialogueText(text)) {
		return Command{}, ErrNeedsItem
	}
	for _, bad := range p.grammar.NoiseFragments {
		if strings.Contains(text, bad) {
			return Command{}, ErrUnclear
		}
	}
	text = p.additionPrefix(text)
	for _, prefix := range p.grammar.WakePrefixes {
		text = strings.TrimPrefix(text, prefix+" ")
	}
	for _, prefix := range p.grammar.CommandPrefixes {
		if strings.HasPrefix(text, prefix+" ") {
			text = strings.TrimPrefix(text, prefix+" ")
			break
		}
	}
	for {
		previous := text
		for _, suffix := range p.grammar.Suffixes {
			text = strings.TrimSuffix(text, " "+suffix)
		}
		if text == previous {
			break
		}
	}
	text = p.separateAttachedUnit(text)
	words := strings.Fields(text)
	for i := range words {
		words[i] = strings.Trim(words[i], ",;:")
	}
	quantity, n, err := p.amount(words)
	if err != nil {
		return Command{}, err
	}
	words = words[n:]
	unit := "piece"
	if len(words) > 0 {
		if u, ok := p.units[strings.TrimSuffix(words[0], ".")]; ok {
			unit = u
			words = words[1:]
		}
	}
	if len(words) > 0 && contains(p.grammar.ItemPrepositions, words[0]) {
		words = words[1:]
	}
	name := strings.Join(words, " ")
	hasLetter := false
	for _, r := range name {
		hasLetter = hasLetter || unicode.IsLetter(r)
	}
	if contains(p.grammar.CommandVerbs, name) || !hasLetter || len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		return Command{}, ErrUnclear
	}
	for _, bad := range p.grammar.RejectedPrefixes {
		if strings.HasPrefix(name, bad+" ") {
			return Command{}, ErrUnclear
		}
	}
	return Command{Text: name, Quantity: quantity, Unit: unit}, nil
}
func (c Command) Reply() string { return c.ReplyLocalized("en") }
func (c Command) ReplyLocalized(language string) string {
	return fmt.Sprintf(i18n.Text(language, "Added %s %s of %s to the shopping list."), i18n.Number(language, c.Quantity), i18n.Unit(language, c.Unit, c.Quantity), c.Text)
}
