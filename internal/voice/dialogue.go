package voice

import (
	"strings"
	"unicode"
)

func contains(choices []string, text string) bool {
	for _, choice := range choices {
		if choice == text {
			return true
		}
	}
	return false
}

func (p *Parser) dialogueText(text string) string {
	text = strings.ToLower(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return ' '
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	for _, name := range p.grammar.Names {
		text = strings.TrimSuffix(text, " "+name)
	}
	return text
}

// Match complete replies. Product names must not be mistaken for dialogue.
func (p *Parser) Ending(text string) bool { return contains(p.grammar.Endings, p.dialogueText(text)) }
func Ending(text string) bool             { return New("en").Ending(text) }

// Ambiguous replies never approve changes to a list.
func (p *Parser) Confirmation(text string) (bool, bool) {
	if p.Ending(text) {
		return false, true
	}
	if contains(p.grammar.Confirmations, p.dialogueText(text)) {
		return true, true
	}
	return false, false
}
func Confirmation(text string) (bool, bool) { return New("en").Confirmation(text) }

func (p *Parser) additionPrefix(text string) string {
	// Strip an affirmative only before a recognizable amount or command verb.
	for _, prefix := range p.grammar.AffirmativePrefixes {
		for _, separator := range []string{" ", ", "} {
			if !strings.HasPrefix(text, prefix+separator) {
				continue
			}
			rest := p.separateAttachedUnit(strings.TrimSpace(strings.TrimPrefix(text, prefix+separator)))
			words := strings.Fields(rest)
			if len(words) == 0 {
				return text
			}
			_, numberWord := p.integer(words[0])
			_, fraction := p.grammar.Fractions[words[0]]
			if decimal.MatchString(words[0]) || numberWord || fraction || contains(p.grammar.CommandVerbs, words[0]) {
				return rest
			}
		}
	}
	return text
}
