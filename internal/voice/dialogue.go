package voice

import (
	"strings"
	"unicode"
)

func dialogueText(text string) string {
	text = strings.ToLower(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return ' '
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	for _, name := range []string{" foodie", " foody"} {
		text = strings.TrimSuffix(text, name)
	}
	return text
}

// Match complete replies, never substrings: "nejliker" remains a grocery.
func Ending(text string) bool {
	switch dialogueText(text) {
	case "no thanks", "no thank you", "nothing else", "that s all", "thats all", "that is all", "all done", "i m done", "goodbye", "cancel", "thank you", "thanks", "not now", "nej", "nej tak", "nejtak", "nej nej", "nej ellers tak", "nej ikke mere", "nej ikke andet", "nej det var det", "nej tak det var det", "nej det var alt", "nej det var det hele",
		"næ", "næh", "næ tak", "næh tak", "næ ikke mere", "nix", "niks", "no", "nope",
		"ellers tak", "ikke noget", "ingenting", "ingen ting", "ikke mere", "ikke andet", "ikke flere", "intet",
		"det var det", "det var alt", "det var bare det", "det var det hele", "det var vist det", "det var alt for nu", "det er alt", "det er det", "det er fint", "det er nok", "det er nok for nu",
		"jeg er færdig", "vi er færdige", "færdig", "slut", "stop", "stop så", "annuller", "annullér", "glem det", "lad være",
		"farvel", "hej hej", "hejhej", "vi ses", "hav en god dag", "tak", "mange tak", "tak for hjælpen", "tak for i dag", "bye", "bye bye":
		return true
	default:
		return false
	}
}

func wantsAnother(text string) bool {
	switch dialogueText(text) {
	case "yes please", "sure", "yeah", "yep", "ja", "ja tak", "jo", "jo tak", "gerne", "ja gerne", "meget gerne", "jep", "jeps", "yes", "ja der var en ting mere":
		return true
	default:
		return false
	}
}

func additionPrefix(text string) string {
	// A natural reply such as "ja, en liter mælk" is still a single addition.
	// Only strip an affirmative before a recognizable amount or command verb.
	for _, prefix := range []string{"yes ", "yes, ", "yeah ", "yeah, ", "ja ", "ja, ", "jo ", "jo, "} {
		if !strings.HasPrefix(text, prefix) {
			continue
		}
		rest := separateAttachedUnit(strings.TrimSpace(strings.TrimPrefix(text, prefix)))
		words := strings.Fields(rest)
		if len(words) == 0 {
			return text
		}
		_, numberWord := integer(words[0])
		if decimal.MatchString(words[0]) || numberWord || words[0] == "add" || words[0] == "half" || words[0] == "tilføj" || words[0] == "skriv" || words[0] == "halvanden" || words[0] == "halvandet" || words[0] == "halv" || words[0] == "halvt" {
			return rest
		}
	}
	return text
}
