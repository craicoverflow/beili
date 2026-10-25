package handlers

import (
	"regexp"
	"strings"
	"unicode"
)

// shoppingItemRe matches a leading quantity at the start of an ingredient
// string and captures the amount, an optional unit and the remaining name. The
// amount is an integer or decimal, optionally a simple range ("10 or 11",
// "4-5"). Anything without a leading numeric amount does not match and is left
// verbatim.
var shoppingItemRe = regexp.MustCompile(`^(\d+(?:\.\d+)?(?:\s*(?:-|–|to|or)\s*\d+(?:\.\d+)?)?)\s*(kg|g|mg|ml|cl|dl|l|cloves?|cans?|tins?|packs?|packets?|slices?|bunch(?:es)?|handfuls?|pinch(?:es)?|sticks?|sprigs?|stalks?|rashers?|fillets?)?\b\s*(?:of\s+)?(.+)$`)

// metricUnits are written tight to the number ("500g"); other units are
// words and stay spaced ("2 cloves").
var metricUnits = map[string]bool{"kg": true, "g": true, "mg": true, "ml": true, "cl": true, "dl": true, "l": true}

// splitShoppingItem splits a stored ingredient string into a capitalised item
// name and a separate amount, e.g. "625g mushrooms" -> ("Mushrooms", "625g").
// Prep notes after the first comma or parenthesis are dropped from the name
// ("2 onions, diced" -> "Onions").
//
// Ingredients with no parseable leading amount (e.g. "salt to taste") are
// returned verbatim as the name (prep note dropped) with an empty amount.
// Stored lines are canonical (see internal/ingredient), so this is exact for
// anything imported; hand-typed lines are parsed on a best-effort basis.
func splitShoppingItem(raw string) (name, amount string) {
	s := strings.TrimSpace(raw)
	m := shoppingItemRe.FindStringSubmatch(s)
	if m == nil {
		return trimPrep(s), "" // no leading amount
	}

	number := strings.Join(strings.Fields(m[1]), " ") // "10  or  11" -> "10 or 11"
	unit := strings.ToLower(m[2])
	n := trimPrep(m[3])
	if n == "" {
		return raw, ""
	}
	switch {
	case unit == "":
	case metricUnits[unit]:
		number += unit
	default:
		number += " " + unit
	}
	return capitalizeFirst(n), number
}

// trimPrep drops everything from the first comma or opening parenthesis.
func trimPrep(s string) string {
	if i := strings.IndexAny(s, ",("); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// formatShoppingItem reformats a stored ingredient string into the shopping
// list form "Name (amount)", e.g. "625g mushrooms" -> "Mushrooms (625g)" and
// "3 carrots, sliced" -> "Carrots (3)". Items with no amount are just the name.
func formatShoppingItem(raw string) string {
	name, amount := splitShoppingItem(raw)
	if amount == "" {
		return capitalizeFirst(name)
	}
	return name + " (" + amount + ")"
}

// capitalizeFirst upper-cases the first rune of s, leaving the rest untouched.
func capitalizeFirst(s string) string {
	for _, r := range s {
		return string(unicode.ToUpper(r)) + s[len(string(r)):]
	}
	return s
}
