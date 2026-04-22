package search

import (
	"strings"
	"unicode"
)

// ParseFTSQuery converts user input into a valid FTS5 boolean expression.
//
// Rules:
//   - Whitespace-separated terms → implicit AND with prefix matching (chicken broccoli → chicken* AND broccoli*)
//   - OR keyword → FTS5 OR (chicken OR beef → chicken* OR beef*)
//   - Quoted phrases → exact match ("olive oil" → "olive oil")
//   - Parentheses for grouping → passed through to FTS5
func ParseFTSQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}

	tokens := tokenize(q)
	if len(tokens) == 0 {
		return ""
	}

	var out []string
	prevWasTerm := false

	for _, tok := range tokens {
		upper := strings.ToUpper(tok)
		switch {
		case upper == "OR":
			out = append(out, "OR")
			prevWasTerm = false
		case upper == "AND":
			out = append(out, "AND")
			prevWasTerm = false
		case tok == "(" || tok == ")":
			out = append(out, tok)
			prevWasTerm = tok == ")"
		case strings.HasPrefix(tok, `"`):
			// Quoted phrase — sanitize inner content but keep as phrase
			inner := strings.Trim(tok, `"`)
			inner = sanitizeWord(inner)
			if inner != "" {
				if prevWasTerm {
					out = append(out, "AND")
				}
				out = append(out, `"`+inner+`"`)
				prevWasTerm = true
			}
		default:
			word := sanitizeWord(tok)
			if word != "" {
				if prevWasTerm {
					out = append(out, "AND")
				}
				out = append(out, word+"*")
				prevWasTerm = true
			}
		}
	}

	return strings.Join(out, " ")
}

// tokenize splits input into tokens respecting quoted strings.
func tokenize(q string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false

	for _, r := range q {
		switch {
		case r == '"':
			if inQuote {
				cur.WriteRune(r)
				tokens = append(tokens, cur.String())
				cur.Reset()
				inQuote = false
			} else {
				if cur.Len() > 0 {
					tokens = append(tokens, cur.String())
					cur.Reset()
				}
				cur.WriteRune(r)
				inQuote = true
			}
		case inQuote:
			cur.WriteRune(r)
		case r == '(' || r == ')':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			tokens = append(tokens, string(r))
		case unicode.IsSpace(r):
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// sanitizeWord removes characters that have special meaning in FTS5.
func sanitizeWord(w string) string {
	var b strings.Builder
	for _, r := range w {
		if r != '*' && r != ':' && r != '-' && r != '^' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
