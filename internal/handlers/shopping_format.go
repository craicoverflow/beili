package handlers

import (
	"regexp"
	"strings"
	"unicode"
)

// shoppingItemRe matches a leading metric quantity at the start of an ingredient
// string and captures the amount and the remaining name. The amount is an
// integer or decimal, optionally a simple range ("10 or 11", "4-5"), optionally
// followed by a known metric unit. Anything without a leading numeric amount
// does not match and is left verbatim.
var shoppingItemRe = regexp.MustCompile(`^(\d+(?:\.\d+)?(?:\s*(?:-|–|to|or)\s*\d+(?:\.\d+)?)?)\s*(kg|g|mg|ml|cl|dl|l)?\b\s*(.+)$`)

// formatShoppingItem reformats a stored ingredient string into a shopping-list
// friendly "Name (amount)" form, e.g. "625g mushrooms" -> "Mushrooms (625g)".
//
// Ingredients with no parseable leading amount (e.g. "salt to taste") are
// returned completely verbatim. The parser only recognises metric units, which
// is safe because recipes are normalised to metric on save.
func formatShoppingItem(raw string) string {
	s := strings.TrimSpace(raw)
	m := shoppingItemRe.FindStringSubmatch(s)
	if m == nil {
		return raw // no leading amount — leave verbatim
	}

	number := strings.TrimSpace(m[1])
	unit := m[2]
	name := strings.TrimSpace(m[3])
	if name == "" {
		return raw
	}

	// Normalise internal whitespace in a range ("10  or  11" -> "10 or 11").
	// A trailing unit is concatenated tight to the number ("240 ml" -> "240ml").
	number = strings.Join(strings.Fields(number), " ")
	amount := number + unit // "240"+"ml", "625"+"g", or "2"+""

	return capitalizeFirst(name) + " (" + amount + ")"
}

// capitalizeFirst upper-cases the first rune of s, leaving the rest untouched.
func capitalizeFirst(s string) string {
	for _, r := range s {
		return string(unicode.ToUpper(r)) + s[len(string(r)):]
	}
	return s
}
