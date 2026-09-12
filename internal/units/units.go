// Package units defines language-independent identifiers for quantities.
package units

const Default = "piece"

var IDs = []string{"piece", "liter", "milliliter", "kilogram", "gram", "pack", "bag", "can", "bottle", "bunch", "tray", "crate"}

func Valid(id string) bool {
	for _, candidate := range IDs {
		if id == candidate {
			return true
		}
	}
	return false
}
