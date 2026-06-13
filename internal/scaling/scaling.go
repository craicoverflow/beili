// Package scaling deterministically scales quantities in recipe ingredient
// and instruction text by a serving ratio. It understands decimals, ASCII and
// unicode fractions, mixed numbers, ranges, and common cooking units, and it
// deliberately leaves temperatures, times, dimensions and reference numbers
// (e.g. "Note 3", "gas 4") untouched.
package scaling

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ScaleIngredients scales every line as an ingredient.
func ScaleIngredients(lines []string, ratio float64) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ScaleIngredient(l, ratio)
	}
	return out
}

// ScaleInstructions scales every line as an instruction step.
func ScaleInstructions(lines []string, ratio float64) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ScaleInstruction(l, ratio)
	}
	return out
}

// ScaleIngredient scales quantities in a single ingredient line. The leading
// quantity is always scaled (e.g. "2 eggs"); quantities elsewhere in the line
// are scaled only when followed by a recognised unit.
func ScaleIngredient(line string, ratio float64) string {
	return scaleLine(line, ratio, true)
}

// ScaleInstruction scales quantities in an instruction step. Only quantities
// followed by a recognised unit are scaled, so step numbers, batch counts and
// similar are left alone.
func ScaleInstruction(line string, ratio float64) string {
	return scaleLine(line, ratio, false)
}

// --- unit tables ---

type unitClass int

const (
	classNone unitClass = iota
	classCount
	classQuarters // tsp/tbsp/cup style: round to kitchen fractions
	classOunces   // oz style: round to halves, decimal display
	classMetric
)

type unitInfo struct {
	class unitClass
	// metricBase is "g" or "ml" for metric units; multiplier converts the
	// written value into that base unit.
	metricBase string
	multiplier float64
}

var units = map[string]unitInfo{
	"g": {class: classMetric, metricBase: "g", multiplier: 1},
	"gram": {class: classMetric, metricBase: "g", multiplier: 1}, "grams": {class: classMetric, metricBase: "g", multiplier: 1},
	"kg": {class: classMetric, metricBase: "g", multiplier: 1000},
	"kilogram": {class: classMetric, metricBase: "g", multiplier: 1000}, "kilograms": {class: classMetric, metricBase: "g", multiplier: 1000},
	"mg": {class: classMetric, metricBase: "g", multiplier: 0.001},
	"ml": {class: classMetric, metricBase: "ml", multiplier: 1},
	"millilitre": {class: classMetric, metricBase: "ml", multiplier: 1}, "millilitres": {class: classMetric, metricBase: "ml", multiplier: 1},
	"milliliter": {class: classMetric, metricBase: "ml", multiplier: 1}, "milliliters": {class: classMetric, metricBase: "ml", multiplier: 1},
	"cl": {class: classMetric, metricBase: "ml", multiplier: 10},
	"dl": {class: classMetric, metricBase: "ml", multiplier: 100},
	"l":  {class: classMetric, metricBase: "ml", multiplier: 1000},
	"litre": {class: classMetric, metricBase: "ml", multiplier: 1000}, "litres": {class: classMetric, metricBase: "ml", multiplier: 1000},
	"liter": {class: classMetric, metricBase: "ml", multiplier: 1000}, "liters": {class: classMetric, metricBase: "ml", multiplier: 1000},

	"tsp": {class: classQuarters}, "tsps": {class: classQuarters},
	"teaspoon": {class: classQuarters}, "teaspoons": {class: classQuarters},
	"tbsp": {class: classQuarters}, "tbsps": {class: classQuarters},
	"tablespoon": {class: classQuarters}, "tablespoons": {class: classQuarters},
	"cup": {class: classQuarters}, "cups": {class: classQuarters},
	"pint": {class: classQuarters}, "pints": {class: classQuarters},
	"quart": {class: classQuarters}, "quarts": {class: classQuarters},
	"gallon": {class: classQuarters}, "gallons": {class: classQuarters},
	"lb": {class: classQuarters}, "lbs": {class: classQuarters},
	"pound": {class: classQuarters}, "pounds": {class: classQuarters},

	"oz": {class: classOunces}, "ounce": {class: classOunces}, "ounces": {class: classOunces},

	"clove": {class: classCount}, "cloves": {class: classCount},
	"can": {class: classCount}, "cans": {class: classCount},
	"tin": {class: classCount}, "tins": {class: classCount},
	"pack": {class: classCount}, "packs": {class: classCount},
	"packet": {class: classCount}, "packets": {class: classCount},
	"slice": {class: classCount}, "slices": {class: classCount},
	"bunch": {class: classCount}, "bunches": {class: classCount},
	"handful": {class: classCount}, "handfuls": {class: classCount},
	"pinch": {class: classCount}, "pinches": {class: classCount},
	"stick": {class: classCount}, "sticks": {class: classCount},
	"sprig": {class: classCount}, "sprigs": {class: classCount},
	"stalk": {class: classCount}, "stalks": {class: classCount},
	"rasher": {class: classCount}, "rashers": {class: classCount},
	"fillet": {class: classCount}, "fillets": {class: classCount},
	"egg": {class: classCount}, "eggs": {class: classCount},
}

// words after a number that mean "do not scale this number"
var followGuards = map[string]bool{
	"min": true, "mins": true, "minute": true, "minutes": true,
	"hour": true, "hours": true, "hr": true, "hrs": true,
	"sec": true, "secs": true, "second": true, "seconds": true,
	"day": true, "days": true, "week": true, "weeks": true,
	"month": true, "months": true, "year": true, "years": true,
	"cm": true, "mm": true, "centimetre": true, "centimetres": true,
	"centimeter": true, "centimeters": true, "millimetre": true,
	"millimetres": true, "millimeter": true, "millimeters": true,
	"inch": true, "inches": true, "in": true,
	"degree": true, "degrees": true, "celsius": true, "fahrenheit": true,
	"percent": true,
}

// words before a number that mean "do not scale this number"
var prevGuards = map[string]bool{
	"note": true, "step": true, "gas": true, "mark": true, "x": true, "×": true,
	"type": true, "no": true, "number": true,
}

// words before a number that force scaling it as a count (in any mode)
var prevForceCount = map[string]bool{
	"serves": true, "makes": true, "feeds": true,
}

// words allowed before a leading quantity in an ingredient line
var leadingPrefixes = map[string]bool{
	"about": true, "approx": true, "approximately": true,
	"around": true, "roughly": true, "scant": true, "heaped": true, "heaping": true,
}

// container words following a metric size mean the number is a package size
// ("400 g can"), which should not itself be scaled.
var containerWords = map[string]bool{
	"can": true, "cans": true, "tin": true, "tins": true,
	"pack": true, "packs": true, "packet": true, "packets": true,
	"jar": true, "jars": true, "bottle": true, "bottles": true,
	"block": true, "blocks": true, "ball": true, "balls": true,
	"bag": true, "bags": true, "box": true, "boxes": true,
	"carton": true, "cartons": true, "pot": true, "pots": true,
	"tub": true, "tubs": true, "sachet": true, "sachets": true,
}

// pluralPairs maps singular unit/noun words to plurals so the word can be
// adjusted to match the scaled quantity.
var pluralPairs = map[string]string{
	"cup": "cups", "teaspoon": "teaspoons", "tablespoon": "tablespoons",
	"ounce": "ounces", "pound": "pounds",
	"litre": "litres", "liter": "liters", "gram": "grams", "kilogram": "kilograms",
	"pint": "pints", "quart": "quarts", "gallon": "gallons",
	"clove": "cloves", "can": "cans", "tin": "tins", "pack": "packs",
	"packet": "packets", "slice": "slices", "bunch": "bunches",
	"handful": "handfuls", "pinch": "pinches", "stick": "sticks",
	"sprig": "sprigs", "stalk": "stalks", "rasher": "rashers",
	"fillet": "fillets", "egg": "eggs",
	"onion": "onions", "carrot": "carrots", "pepper": "peppers",
	"tomato": "tomatoes", "potato": "potatoes", "shallot": "shallots",
	"lemon": "lemons", "lime": "limes", "apple": "apples", "banana": "bananas",
}

var singularPairs = func() map[string]string {
	m := make(map[string]string, len(pluralPairs))
	for s, p := range pluralPairs {
		m[p] = s
	}
	return m
}()

// --- number parsing ---

var vulgarFractions = map[rune]float64{
	'¼': 0.25, '½': 0.5, '¾': 0.75,
	'⅓': 1.0 / 3, '⅔': 2.0 / 3,
	'⅕': 0.2, '⅖': 0.4, '⅗': 0.6, '⅘': 0.8,
	'⅙': 1.0 / 6, '⅚': 5.0 / 6,
	'⅐': 1.0 / 7, '⅑': 1.0 / 9, '⅒': 0.1,
	'⅛': 0.125, '⅜': 0.375, '⅝': 0.625, '⅞': 0.875,
}

const vulgarClass = "¼½¾⅓⅔⅕⅖⅗⅘⅙⅚⅐⅑⅒⅛⅜⅝⅞"

// Ordered alternatives: mixed ascii fraction, ascii fraction, number with
// attached vulgar fraction, bare vulgar fraction, decimal/integer.
var numRe = regexp.MustCompile(`\d+\s+\d+\s*/\s*\d+|\d+\s*/\s*\d+|\d+(?:\.\d+)?\s?[` + vulgarClass + `]|[` + vulgarClass + `]|\d+(?:\.\d+)?`)

var rangeSepRe = regexp.MustCompile(`^(\s*(?:-|–|—|to|or)\s*)$`)

func parseNumber(s string) float64 {
	s = strings.TrimSpace(s)
	// trailing vulgar fraction (possibly with a leading whole number)
	runes := []rune(s)
	if v, ok := vulgarFractions[runes[len(runes)-1]]; ok {
		whole := strings.TrimSpace(string(runes[:len(runes)-1]))
		if whole == "" {
			return v
		}
		n, _ := strconv.ParseFloat(whole, 64)
		return n + v
	}
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		den, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		numPart := strings.TrimSpace(parts[0])
		whole := 0.0
		if fields := strings.Fields(numPart); len(fields) == 2 {
			whole, _ = strconv.ParseFloat(fields[0], 64)
			numPart = fields[1]
		}
		num, _ := strconv.ParseFloat(numPart, 64)
		if den == 0 {
			return whole + num
		}
		return whole + num/den
	}
	n, _ := strconv.ParseFloat(s, 64)
	return n
}

// --- scanning ---

type span struct {
	start, end int
	text       string
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// wordAfter returns the word (letters only) starting at or after pos,
// skipping leading spaces, plus its start/end offsets.
func wordAfter(s string, pos int) (string, int, int) {
	for pos < len(s) && s[pos] == ' ' {
		pos++
	}
	// unicode × guard
	if strings.HasPrefix(s[pos:], "×") {
		return "×", pos, pos + len("×")
	}
	start := pos
	for pos < len(s) && isLetter(s[pos]) {
		pos++
	}
	return strings.ToLower(s[start:pos]), start, pos
}

// wordBefore returns the word (letters only) ending immediately before pos,
// skipping trailing spaces.
func wordBefore(s string, pos int) string {
	for pos > 0 && s[pos-1] == ' ' {
		pos--
	}
	if strings.HasSuffix(s[:pos], "×") {
		return "×"
	}
	end := pos
	for pos > 0 && isLetter(s[pos-1]) {
		pos--
	}
	return strings.ToLower(s[pos:end])
}

type decision struct {
	num      span      // the number (or first number of a range) to replace
	num2     *span     // second number of a range, if any
	sep      string    // range separator text
	class    unitClass // how to format the scaled value
	unit     *span     // unit token to rewrite (pluralise / kg upgrade), if any
	unitInfo unitInfo
	prepend  string // text to insert before the number (container multiplier)
}

func scaleLine(line string, ratio float64, ingredient bool) string {
	if ratio == 1 || line == "" {
		return line
	}
	locs := numRe.FindAllStringIndex(line, -1)
	if len(locs) == 0 {
		return line
	}

	var decisions []decision
	for i := 0; i < len(locs); i++ {
		d := decision{num: span{locs[i][0], locs[i][1], line[locs[i][0]:locs[i][1]]}}

		// range detection: "4-5", "2 or 3", "4 to 5"
		end := d.num.end
		if i+1 < len(locs) {
			between := line[d.num.end:locs[i+1][0]]
			if rangeSepRe.MatchString(between) {
				d.sep = between
				d.num2 = &span{locs[i+1][0], locs[i+1][1], line[locs[i+1][0]:locs[i+1][1]]}
				end = d.num2.end
				i++
			}
		}

		prev := wordBefore(line, d.num.start)
		if prevGuards[prev] {
			continue
		}

		// character guards directly after the number/range: % ° "
		rest := strings.TrimLeft(line[end:], " ")
		if strings.HasPrefix(rest, "%") || strings.HasPrefix(rest, "°") || strings.HasPrefix(rest, "\"") || strings.HasPrefix(rest, "″") {
			continue
		}

		follow, fStart, fEnd := wordAfter(line, end)
		val := parseNumber(d.num.text)
		if val == 0 {
			continue
		}
		if followGuards[follow] {
			continue
		}
		// bare C/F after a large number is a temperature
		if (follow == "c" || follow == "f") && val >= 50 {
			continue
		}

		info, isUnit := units[follow]
		switch {
		case prevForceCount[prev]:
			d.class = classCount
		case isUnit:
			d.class = info.class
			d.unit = &span{fStart, fEnd, line[fStart:fEnd]}
			d.unitInfo = info
			if info.class == classMetric {
				// package-size guard: "400 g can" — the size is fixed
				next, _, _ := wordAfter(line, fEnd)
				if containerWords[next] {
					if ingredient && isLeading(line, d.num.start) {
						d.prepend = formatCount(ratio) + " x "
						d.class = classNone // keep the size itself unscaled
					} else {
						continue
					}
				}
			}
		case ingredient && isLeading(line, d.num.start):
			d.class = classCount
		default:
			continue
		}

		// track a known noun after a bare count so it can be (un)pluralised
		if d.unit == nil && d.class == classCount {
			if _, ok := pluralPairs[follow]; ok {
				d.unit = &span{fStart, fEnd, line[fStart:fEnd]}
			} else if _, ok := singularPairs[follow]; ok {
				d.unit = &span{fStart, fEnd, line[fStart:fEnd]}
			}
		}

		decisions = append(decisions, d)
	}

	if len(decisions) == 0 {
		return line
	}

	// rebuild the line back-to-front so spans stay valid
	out := line
	for i := len(decisions) - 1; i >= 0; i-- {
		out = applyDecision(out, decisions[i], ratio)
	}
	return out
}

// isLeading reports whether everything before pos is whitespace or an allowed
// prefix word ("about 2 cups").
func isLeading(line string, pos int) bool {
	prefix := strings.TrimSpace(line[:pos])
	if prefix == "" {
		return true
	}
	for w := range strings.FieldsSeq(strings.ToLower(prefix)) {
		if !leadingPrefixes[w] {
			return false
		}
	}
	return true
}

func applyDecision(line string, d decision, ratio float64) string {
	if d.class == classNone {
		// container size: just prepend the multiplier
		return line[:d.num.start] + d.prepend + line[d.num.start:]
	}

	var newNum, newNum2, newUnit string
	switch d.class {
	case classMetric:
		base := parseNumber(d.num.text) * d.unitInfo.multiplier * ratio
		newNum, newUnit = formatMetric(base, d.unitInfo.metricBase)
		if d.num2 != nil {
			base2 := parseNumber(d.num2.text) * d.unitInfo.multiplier * ratio
			n2, u2 := formatMetric(base2, d.unitInfo.metricBase)
			newNum2 = n2
			newUnit = u2 // unit follows the larger (second) value
		}
	case classQuarters:
		newNum = formatQuarters(parseNumber(d.num.text) * ratio)
		if d.num2 != nil {
			newNum2 = formatQuarters(parseNumber(d.num2.text) * ratio)
		}
	case classOunces:
		newNum = formatHalvesDecimal(parseNumber(d.num.text) * ratio)
		if d.num2 != nil {
			newNum2 = formatHalvesDecimal(parseNumber(d.num2.text) * ratio)
		}
	default: // classCount
		if d.num2 != nil {
			// ranges of counts read better as whole numbers
			newNum = formatWhole(parseNumber(d.num.text) * ratio)
			newNum2 = formatWhole(parseNumber(d.num2.text) * ratio)
		} else {
			newNum = formatCount(parseNumber(d.num.text) * ratio)
		}
	}

	// rewrite unit token: metric upgrade (g→kg) or pluralisation
	if d.unit != nil {
		tok := d.unit.text
		replacement := tok
		if d.class == classMetric && newUnit != "" && newUnit != canonicalShort(d.unitInfo) {
			replacement = newUnit
		} else if d.class != classMetric {
			val := parseNumber(newNum)
			if d.num2 != nil {
				val = parseNumber(newNum2)
			}
			lower := strings.ToLower(tok)
			if val > 1 {
				if p, ok := pluralPairs[lower]; ok {
					replacement = p
				}
			} else {
				if s, ok := singularPairs[lower]; ok {
					replacement = s
				}
			}
		}
		line = line[:d.unit.start] + replacement + line[d.unit.end:]
	}

	if d.num2 != nil {
		line = line[:d.num2.start] + newNum2 + line[d.num2.end:]
	}
	return line[:d.num.start] + newNum + line[d.num.end:]
}

// canonicalShort returns the abbreviated unit a unitInfo represents, so a
// token is only rewritten when scaling actually changes the unit.
func canonicalShort(info unitInfo) string {
	switch {
	case info.metricBase == "g" && info.multiplier == 1000:
		return "kg"
	case info.metricBase == "g" && info.multiplier == 0.001:
		return "mg"
	case info.metricBase == "g":
		return "g"
	case info.multiplier == 1000:
		return "l"
	case info.multiplier == 100:
		return "dl"
	case info.multiplier == 10:
		return "cl"
	default:
		return "ml"
	}
}

// --- formatting ---

func trimFloat(v float64) string {
	v = math.Round(v*10000) / 10000
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// formatMetric renders a value in base units (g or ml) using kitchen-sensible
// rounding, upgrading to kg/l above 1000. It returns the number and the unit.
func formatMetric(base float64, baseUnit string) (string, string) {
	bigUnit := "kg"
	if baseUnit == "ml" {
		bigUnit = "l"
	}
	switch {
	case base >= 1000:
		return trimFloat(math.Round(base/100) / 10), bigUnit
	case base >= 100:
		return trimFloat(math.Round(base/10) * 10), baseUnit
	case base >= 10:
		return trimFloat(math.Round(base/5) * 5), baseUnit
	case base >= 1:
		return trimFloat(math.Round(base*2) / 2), baseUnit
	default:
		v := math.Round(base*10) / 10
		if v < 0.1 {
			v = 0.1
		}
		return trimFloat(v), baseUnit
	}
}

var fractionGlyphs = map[float64]string{
	0.125: "⅛", 0.25: "¼", 0.375: "⅜", 0.5: "½",
	0.625: "⅝", 0.75: "¾", 0.875: "⅞",
}

// formatQuarters snaps to kitchen fractions (quarters, or eighths for tiny
// amounts) and renders with unicode fractions: "1½", "¾", "3".
func formatQuarters(v float64) string {
	var snapped float64
	if v < 0.1875 { // closer to ⅛ than ¼
		snapped = math.Round(v*8) / 8
		if snapped < 0.125 {
			snapped = 0.125
		}
	} else {
		snapped = math.Round(v*4) / 4
	}
	whole := math.Floor(snapped)
	frac := snapped - whole
	glyph := fractionGlyphs[frac]
	switch {
	case frac == 0:
		return trimFloat(whole)
	case whole == 0:
		return glyph
	default:
		return trimFloat(whole) + glyph
	}
}

// formatCount snaps to halves and renders with a unicode half: "2½", "3".
func formatCount(v float64) string {
	snapped := math.Round(v*2) / 2
	if snapped < 0.5 {
		snapped = 0.5
	}
	whole := math.Floor(snapped)
	if snapped == whole {
		return trimFloat(whole)
	}
	if whole == 0 {
		return "½"
	}
	return trimFloat(whole) + "½"
}

// formatWhole rounds to the nearest whole number, minimum 1.
func formatWhole(v float64) string {
	n := math.Round(v)
	if n < 1 {
		n = 1
	}
	return trimFloat(n)
}

// formatHalvesDecimal snaps to halves and renders as a decimal: "13.5".
func formatHalvesDecimal(v float64) string {
	snapped := math.Round(v*2) / 2
	if snapped < 0.5 {
		snapped = 0.5
	}
	return trimFloat(snapped)
}
