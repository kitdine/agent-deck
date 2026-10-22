package p07measure

import "strings"

// The CTE recognizer admits only complete WITH declarations whose bodies and
// terminal statement are SELECT/VALUES queries. It does not classify WITH by
// its prefix alone. Unsupported/malformed shapes stay unknown.
type cteToken struct {
	text       string
	identifier bool
	quoted     bool
}

func (t cteToken) keyword(word string) bool { return !t.quoted && t.text == word }

func cteTokens(query string) ([]cteToken, bool) {
	var tokens []cteToken
	for i := 0; i < len(query); {
		c := query[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' {
			i++
			continue
		}
		if c == '-' && i+1 < len(query) && query[i+1] == '-' {
			i += 2
			for i < len(query) && query[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(query) && query[i+1] == '*' {
			end := strings.Index(query[i+2:], "*/")
			if end < 0 {
				return nil, false
			}
			i += end + 4
			continue
		}
		if c == '\'' || c == '"' || c == '`' || c == '[' {
			end := c
			if c == '[' {
				end = ']'
			}
			i++
			closed := false
			for i < len(query) {
				if query[i] == end {
					i++
					if c != '[' && i < len(query) && query[i] == end {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, false
			}
			tokens = append(tokens, cteToken{identifier: c != '\'', quoted: true})
			continue
		}
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 128 {
			start := i
			i++
			ascii := c < 128
			for i < len(query) {
				b := query[i]
				if !(b == '_' || b == '$' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b >= 128) {
					break
				}
				ascii = ascii && b < 128
				i++
			}
			word := query[start:i]
			if ascii {
				word = strings.ToUpper(word)
			}
			tokens = append(tokens, cteToken{text: word, identifier: true})
			continue
		}
		tokens = append(tokens, cteToken{text: string(c)})
		i++
	}
	return tokens, true
}

func cteIdentifier(t cteToken) bool {
	if !t.identifier {
		return false
	}
	if t.quoted {
		return true
	}
	switch t.text {
	case "WITH", "RECURSIVE", "AS", "SELECT", "VALUES", "INSERT", "UPDATE", "DELETE", "REPLACE", "CREATE", "ALTER", "DROP", "PRAGMA", "BEGIN", "END", "NOT":
		return false
	}
	return true
}

func cteClosingParen(tokens []cteToken, open int) int {
	if open >= len(tokens) || tokens[open].text != "(" {
		return -1
	}
	depth := 0
	for i := open; i < len(tokens); i++ {
		switch tokens[i].text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func cteBalanced(tokens []cteToken) bool {
	depth := 0
	for _, t := range tokens {
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func cteReadQuery(tokens []cteToken, depth int) bool {
	if depth >= 64 || len(tokens) == 0 || !cteBalanced(tokens) {
		return false
	}
	if tokens[0].keyword("SELECT") || tokens[0].keyword("VALUES") {
		return true
	}
	if !tokens[0].keyword("WITH") {
		return false
	}
	i := 1
	if i < len(tokens) && tokens[i].keyword("RECURSIVE") {
		i++
	}
	for {
		if i >= len(tokens) || !cteIdentifier(tokens[i]) {
			return false
		}
		i++
		if i < len(tokens) && tokens[i].text == "(" {
			i++
			for {
				if i >= len(tokens) || !cteIdentifier(tokens[i]) {
					return false
				}
				i++
				if i < len(tokens) && tokens[i].text == "," {
					i++
					continue
				}
				break
			}
			if i >= len(tokens) || tokens[i].text != ")" {
				return false
			}
			i++
		}
		if i >= len(tokens) || !tokens[i].keyword("AS") {
			return false
		}
		i++
		if i < len(tokens) && tokens[i].keyword("NOT") {
			i++
			if i >= len(tokens) || !tokens[i].keyword("MATERIALIZED") {
				return false
			}
			i++
		} else if i < len(tokens) && tokens[i].keyword("MATERIALIZED") {
			i++
		}
		end := cteClosingParen(tokens, i)
		if end < 0 || !cteReadQuery(tokens[i+1:end], depth+1) {
			return false
		}
		i = end + 1
		if i < len(tokens) && tokens[i].text == "," {
			i++
			continue
		}
		break
	}
	return i < len(tokens) && (tokens[i].keyword("SELECT") || tokens[i].keyword("VALUES"))
}

func readOnlyCTE(query string) bool {
	tokens, ok := cteTokens(query)
	if !ok || len(tokens) == 0 {
		return false
	}
	if tokens[len(tokens)-1].text == ";" {
		tokens = tokens[:len(tokens)-1]
	}
	for _, t := range tokens {
		if t.text == ";" {
			return false
		}
	}
	return len(tokens) > 0 && tokens[0].keyword("WITH") && cteReadQuery(tokens, 0)
}
