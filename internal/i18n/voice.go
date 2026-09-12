package i18n

// VoiceGrammar is selected as a whole. Recognition never merges language packs.
// Unit aliases map spoken words to the same English IDs used in storage and APIs.
type VoiceGrammar struct {
	Names               []string            `json:"names"`
	WakePrefixes        []string            `json:"wake_prefixes"`
	UnitAliases         map[string][]string `json:"unit_aliases"`
	Numbers             map[string]int      `json:"numbers"`
	Fractions           map[string]float64  `json:"fractions"`
	Articles            []string            `json:"articles"`
	Multipliers         map[string]int      `json:"multipliers"`
	Conjunction         string              `json:"conjunction"`
	DecimalWord         string              `json:"decimal_word"`
	NegativeWord        string              `json:"negative_word"`
	CompoundSeparator   string              `json:"compound_separator"`
	CompoundReversed    bool                `json:"compound_reversed"`
	SpacedTens          bool                `json:"spaced_tens"`
	Endings             []string            `json:"endings"`
	Affirmatives        []string            `json:"affirmatives"`
	Confirmations       []string            `json:"confirmations"`
	AffirmativePrefixes []string            `json:"affirmative_prefixes"`
	CommandPrefixes     []string            `json:"command_prefixes"`
	CommandVerbs        []string            `json:"command_verbs"`
	Suffixes            []string            `json:"suffixes"`
	ItemPrepositions    []string            `json:"item_prepositions"`
	NoiseFragments      []string            `json:"noise_fragments"`
	RejectedPrefixes    []string            `json:"rejected_prefixes"`
}
