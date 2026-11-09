// Package revision compares a recipe with an AI-revised version of it. The
// diff is computed here, in code, so the review screen never depends on the
// model describing its own edits accurately.
package revision

import (
	"fmt"
	"strings"
)

// Kind says what happened to an item.
type Kind string

const (
	Added   Kind = "added"
	Removed Kind = "removed"
	Changed Kind = "changed"
)

// Change is one difference between the original and revised recipe. Old is
// empty for Added, New is empty for Removed.
type Change struct {
	Kind   Kind
	Old    string
	New    string
	Reason string
}

// Reason is the model's explanation for a change. Ref is an ingredient name
// or "step N".
type Reason struct {
	Ref  string
	Text string
}

// units that may follow a quantity at the start of an ingredient line.
var units = map[string]bool{
	"g": true, "kg": true, "mg": true, "ml": true, "l": true, "cl": true, "dl": true,
	"tsp": true, "tbsp": true, "cup": true, "cups": true, "floz": true, "pint": true, "pints": true,
	"oz": true, "lb": true, "lbs": true,
	"clove": true, "cloves": true, "can": true, "cans": true, "tin": true, "tins": true,
	"pack": true, "packs": true, "packet": true, "packets": true, "slice": true, "slices": true,
	"bunch": true, "bunches": true, "handful": true, "handfuls": true, "pinch": true, "pinches": true,
	"stick": true, "sticks": true, "sprig": true, "sprigs": true, "stalk": true, "stalks": true,
	"rasher": true, "rashers": true, "fillet": true, "fillets": true,
}

// NameKey reduces an ingredient line like "30 ml soy sauce, to taste" to the
// lower-case product name "soy sauce", so the same ingredient can be matched
// across a change of quantity or prep.
func NameKey(line string) string {
	if i := strings.Index(line, ","); i >= 0 {
		line = line[:i]
	}
	words := strings.Fields(strings.ToLower(line))
	for len(words) > 0 && (isNumber(words[0]) || units[words[0]]) {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

func isNumber(w string) bool {
	if w == "" {
		return false
	}
	for _, r := range w {
		if (r < '0' || r > '9') && r != '.' && r != '/' && r != '-' {
			return false
		}
	}
	return true
}

// Ingredients diffs two ingredient lists. Identical lines are unchanged;
// leftover lines are paired by product name (a changed quantity or prep), and
// whatever is still unpaired is removed or added.
func Ingredients(original, revised []string, reasons []Reason) []Change {
	// Drop identical lines (as a multiset, so duplicates are respected).
	left := make([]bool, len(original))
	used := make([]bool, len(revised))
	for i, o := range original {
		for j, r := range revised {
			if !used[j] && o == r {
				left[i], used[j] = true, true
				break
			}
		}
	}

	var out []Change
	for i, o := range original {
		if left[i] {
			continue
		}
		key := NameKey(o)
		paired := -1
		for j, r := range revised {
			if !used[j] && key != "" && NameKey(r) == key {
				paired = j
				break
			}
		}
		if paired >= 0 {
			used[paired] = true
			out = append(out, Change{Kind: Changed, Old: o, New: revised[paired], Reason: findReason(reasons, key)})
			continue
		}
		out = append(out, Change{Kind: Removed, Old: o, Reason: findReason(reasons, key)})
	}
	for j, r := range revised {
		if !used[j] {
			out = append(out, Change{Kind: Added, New: r, Reason: findReason(reasons, NameKey(r))})
		}
	}
	return out
}

// Instructions diffs two step lists. Steps are aligned with a longest common
// subsequence; within each stretch between matching steps, steps are paired
// in order as changed, with any surplus removed or added.
func Instructions(original, revised []string, reasons []Reason) []Change {
	n, m := len(original), len(revised)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if original[i] == revised[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	var out []Change
	flush := func(removed, added []int) {
		for k := 0; k < len(removed) || k < len(added); k++ {
			switch {
			case k < len(removed) && k < len(added):
				out = append(out, Change{Kind: Changed, Old: original[removed[k]], New: revised[added[k]],
					Reason: findReason(reasons, stepRef(added[k]))})
			case k < len(removed):
				out = append(out, Change{Kind: Removed, Old: original[removed[k]],
					Reason: findReason(reasons, stepRef(removed[k]))})
			default:
				out = append(out, Change{Kind: Added, New: revised[added[k]],
					Reason: findReason(reasons, stepRef(added[k]))})
			}
		}
	}

	var removed, added []int
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && original[i] == revised[j]:
			flush(removed, added)
			removed, added = nil, nil
			i++
			j++
		case j >= m || (i < n && lcs[i+1][j] >= lcs[i][j+1]):
			removed = append(removed, i)
			i++
		default:
			added = append(added, j)
			j++
		}
	}
	flush(removed, added)
	return out
}

func stepRef(idx int) string { return fmt.Sprintf("step %d", idx+1) }

// findReason returns the reason whose ref matches (case-insensitively).
func findReason(reasons []Reason, ref string) string {
	for _, r := range reasons {
		if strings.EqualFold(strings.TrimSpace(r.Ref), ref) {
			return r.Text
		}
	}
	return ""
}
