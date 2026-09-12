// Package i18n shares language packs across the web UI, API and Android build.
package i18n

import (
	"embed"
	"encoding/json"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
)

//go:embed locales/*.json
var files embed.FS

type Pack struct {
	DecimalSeparator string               `json:"decimal_separator"`
	Language         string               `json:"language"`
	Locale           string               `json:"locale"`
	Messages         map[string]string    `json:"messages"`
	Units            map[string][2]string `json:"units"`
	Voice            VoiceGrammar         `json:"voice"`
}

var packs = func() map[string]Pack {
	result := map[string]Pack{}
	for _, lang := range []string{"en", "da"} {
		data, err := files.ReadFile("locales/" + lang + ".json")
		if err != nil {
			panic(err)
		}
		var pack Pack
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&pack); err != nil {
			panic(err)
		}
		result[lang] = pack
	}
	if err := validatePacks(result); err != nil {
		panic(err)
	}
	return result
}()

func Get(language string) Pack {
	if pack, ok := packs[language]; ok {
		return pack
	}
	return packs["en"]
}
func Text(language, source string) string {
	if text, ok := Get(language).Messages[source]; ok {
		return text
	}
	return source
}
func Number(language string, n float64) string {
	text := strconv.FormatFloat(math.Round(n*100)/100, 'f', -1, 64)
	text = strings.ReplaceAll(text, ".", Get(language).DecimalSeparator)
	return text
}
func Unit(language, unit string, n float64) string {
	if forms, ok := Get(language).Units[unit]; ok {
		if n == 1 {
			return forms[0]
		}
		return forms[1]
	}
	return unit
}

// JSON exposes presentation data only; voice parsing stays on the server.
func JSON(language string) []byte {
	pack := Get(language)
	data, _ := json.Marshal(struct {
		Language string               `json:"language"`
		Locale   string               `json:"locale"`
		Messages map[string]string    `json:"messages"`
		Units    map[string][2]string `json:"units"`
	}{pack.Language, pack.Locale, pack.Messages, pack.Units})
	return data
}

var textNode = regexp.MustCompile(`>[^<>]+<`)
var attribute = regexp.MustCompile(`(aria-label|title|placeholder)="([^"]*)"`)

// Translate trusted template text and labels, never identifiers or user content.
func HTML(language, source string) string {
	source = strings.Replace(source, `lang="en"`, `lang="`+Get(language).Language+`"`, 1)
	source = textNode.ReplaceAllStringFunc(source, func(node string) string {
		raw := node[1 : len(node)-1]
		trimmed := strings.TrimSpace(raw)
		translated := Text(language, html.UnescapeString(trimmed))
		if translated == html.UnescapeString(trimmed) {
			return node
		}
		return ">" + strings.Replace(raw, trimmed, html.EscapeString(translated), 1) + "<"
	})
	return attribute.ReplaceAllStringFunc(source, func(attr string) string {
		parts := attribute.FindStringSubmatch(attr)
		return parts[1] + `="` + html.EscapeString(Text(language, html.UnescapeString(parts[2]))) + `"`
	})
}
