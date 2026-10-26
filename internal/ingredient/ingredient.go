// Package ingredient defines the canonical stored form of a recipe
// ingredient line and the deterministic rules for producing and reading it.
//
// The AI only parses a free-text line into fields (Parsed); everything that
// affects numbers — unit conversion, rounding, wording — happens here so that
// stored lines are always something internal/scaling can scale exactly:
//
//	"500 g carrots, sliced"   metric amount + name + optional prep note
//	"3 carrots"               bare count
//	"2 cloves garlic"         count unit
//	"Salt, to taste"          no amount
package ingredient

import (
	"math"
	"strconv"
	"strings"

	"github.com/craicoverflow/beili/internal/scaling"
)

// Parsed is a free-text ingredient line broken into fields, with the unit as
// written in the source (e.g. "cup", "oz"). Qty 0 means "no amount".
type Parsed struct {
	Qty  float64
	Unit string
	Name string
	Prep string
}

// metric conversions into g or ml. Values follow the table the app has always
// used for imperial -> metric.
var toMetric = map[string]struct {
	factor float64
	base   string
}{
	"g": {1, "g"}, "gram": {1, "g"}, "grams": {1, "g"},
	"kg": {1000, "g"}, "kilogram": {1000, "g"}, "kilograms": {1000, "g"},
	"mg": {0.001, "g"},
	"ml": {1, "ml"}, "millilitre": {1, "ml"}, "millilitres": {1, "ml"}, "milliliter": {1, "ml"}, "milliliters": {1, "ml"},
	"cl": {10, "ml"}, "dl": {100, "ml"},
	"l": {1000, "ml"}, "litre": {1000, "ml"}, "litres": {1000, "ml"}, "liter": {1000, "ml"}, "liters": {1000, "ml"},
	"tsp": {5, "ml"}, "teaspoon": {5, "ml"}, "teaspoons": {5, "ml"},
	"tbsp": {15, "ml"}, "tablespoon": {15, "ml"}, "tablespoons": {15, "ml"},
	"cup": {240, "ml"}, "cups": {240, "ml"},
	"floz": {30, "ml"}, "fl oz": {30, "ml"},
	"pint": {568, "ml"}, "pints": {568, "ml"},
	"oz": {28, "g"}, "ounce": {28, "g"}, "ounces": {28, "g"},
	"lb": {454, "g"}, "lbs": {454, "g"}, "pound": {454, "g"}, "pounds": {454, "g"},
}

// countUnits are non-metric units kept as words and understood by the scaler.
var countUnits = map[string]string{
	"clove": "cloves", "can": "cans", "tin": "tins", "pack": "packs", "packet": "packets",
	"slice": "slices", "bunch": "bunches", "handful": "handfuls", "pinch": "pinches",
	"stick": "sticks", "sprig": "sprigs", "stalk": "stalks", "rasher": "rashers", "fillet": "fillets",
}

func countPlural(u string) string {
	for s, p := range countUnits {
		if u == s || u == p {
			return p
		}
	}
	return ""
}

func countSingular(u string) string {
	for s, p := range countUnits {
		if u == s || u == p {
			return s
		}
	}
	return ""
}

// Canonical builds the stored ingredient line from a parsed one.
func Canonical(p Parsed) string {
	name := strings.TrimSpace(p.Name)
	prep := strings.TrimSpace(p.Prep)
	// A number in the note would be scaled independently of the real amount
	// (or be left stale), so notes with digits are dropped.
	if strings.ContainsAny(prep, "0123456789¼½¾⅓⅔⅛⅜⅝⅞") {
		prep = ""
	}
	if name == "" {
		return ""
	}
	tail := name
	if prep != "" {
		tail += ", " + prep
	}
	if p.Qty <= 0 {
		return capitalise(tail)
	}

	unit := strings.ToLower(strings.TrimSpace(p.Unit))
	if m, ok := toMetric[unit]; ok {
		num, u := scaling.FormatMetric(p.Qty*m.factor, m.base)
		return num + " " + u + " " + tail
	}
	if countSingular(unit) != "" {
		w := countSingular(unit)
		if p.Qty > 1 {
			w = countPlural(unit)
		}
		return trim(p.Qty) + " " + w + " " + tail
	}
	return trim(p.Qty) + " " + tail
}

func trim(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

func capitalise(s string) string {
	for _, r := range s {
		return strings.ToUpper(string(r)) + s[len(string(r)):]
	}
	return s
}
