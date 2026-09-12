package i18n

import (
	"fmt"
	"strings"

	"github.com/jonasrappy/foodie/internal/units"
)

// A broken language pack must fail at startup, rather than halfway through a
// voice conversation. English messages are the source keys for every pack.
func validatePacks(packs map[string]Pack) error {
	english := packs["en"]
	for key, value := range english.Messages {
		if key != value {
			return fmt.Errorf("English source key differs from its message: %q", key)
		}
	}
	for language, pack := range packs {
		if pack.DecimalSeparator != "." && pack.DecimalSeparator != "," {
			return fmt.Errorf("invalid decimal separator for %s", language)
		}
		if pack.Language != language || !strings.HasPrefix(pack.Locale, language+"-") {
			return fmt.Errorf("invalid locale for %s", language)
		}
		if len(pack.Messages) != len(english.Messages) {
			return fmt.Errorf("incomplete messages for %s", language)
		}
		for key := range english.Messages {
			value, ok := pack.Messages[key]
			if !ok || value == "" || strings.Count(key, "%s") != strings.Count(value, "%s") {
				return fmt.Errorf("invalid translation for %s: %q", language, key)
			}
		}
		if len(pack.Units) != len(units.IDs) || len(pack.Voice.UnitAliases) != len(units.IDs) {
			return fmt.Errorf("incomplete units for %s", language)
		}
		aliases := map[string]bool{}
		for _, id := range units.IDs {
			if forms := pack.Units[id]; forms[0] == "" || forms[1] == "" {
				return fmt.Errorf("missing unit forms for %s: %s", language, id)
			}
			if len(pack.Voice.UnitAliases[id]) == 0 {
				return fmt.Errorf("missing voice unit for %s: %s", language, id)
			}
			for _, alias := range pack.Voice.UnitAliases[id] {
				if aliases[alias] || strings.ContainsAny(alias, " .\n") || alias == "" {
					return fmt.Errorf("invalid voice alias for %s: %q", language, alias)
				}
				aliases[alias] = true
			}
		}
		grammar := pack.Voice
		if len(grammar.Numbers) == 0 || len(grammar.Confirmations) == 0 || len(grammar.Endings) == 0 || grammar.CompoundSeparator == "" || grammar.Conjunction == "" || grammar.DecimalWord == "" {
			return fmt.Errorf("incomplete voice grammar for %s", language)
		}
		for _, confirmation := range grammar.Confirmations {
			for _, ending := range grammar.Endings {
				if confirmation == ending {
					return fmt.Errorf("ambiguous confirmation for %s: %q", language, confirmation)
				}
			}
		}
	}
	return nil
}
