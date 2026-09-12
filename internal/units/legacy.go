package units

// NormalizeLegacy is a storage and old-client compatibility adapter, never a
// speech vocabulary. Releases before 1.4 used these localized IDs on the wire.
func NormalizeLegacy(id string) string {
	switch id {
	case "stk.":
		return "piece"
	case "kilo":
		return "kilogram"
	case "pakker":
		return "pack"
	case "poser":
		return "bag"
	case "dåser":
		return "can"
	case "flasker":
		return "bottle"
	case "bundter":
		return "bunch"
	case "bakker":
		return "tray"
	case "kasser":
		return "crate"
	default:
		return id
	}
}
