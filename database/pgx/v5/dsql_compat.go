package pgx

import (
	"fmt"
	"strings"
)

// postgresDSQL changes only the ASYNC keyword in supported DSQL statement
// prefixes. Preserve byte offsets and newlines for PostgreSQL error reporting.
// This is a test dialect adapter, not an emulator of DSQL's execution semantics.
func postgresDSQL(input []byte) ([]byte, error) {
	sql := string(input)
	out := append([]byte(nil), input...)
	var prefix []string
	for i := 0; i < len(input); {
		start := i
		switch {
		case strings.HasPrefix(sql[i:], "--"):
			for i < len(input) && input[i] != '\n' {
				i++
			}
		case strings.HasPrefix(sql[i:], "/*"):
			i += 2
			depth := 1
			for i < len(input) && depth > 0 {
				switch {
				case strings.HasPrefix(sql[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(sql[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("DSQL compatibility: unterminated comment")
			}
		case input[i] == '\'' || input[i] == '"':
			quote := input[i]
			escaped := quote == '\'' && start > 0 && (input[start-1] == 'e' || input[start-1] == 'E') && (start == 1 || !sqlWord(input[start-2]))
			i++
			closed := false
			for i < len(input) {
				if escaped && input[i] == '\\' {
					i += 2
					continue
				}
				if input[i] == quote {
					i++
					if i < len(input) && input[i] == quote {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("DSQL compatibility: unterminated quoted value")
			}
			if len(prefix) < 5 {
				prefix = append(prefix, "quoted")
			}
		case input[i] == '$':
			j := i + 1
			for j < len(input) && (input[j] == '_' || input[j] >= 'a' && input[j] <= 'z' || input[j] >= 'A' && input[j] <= 'Z' || j > i+1 && input[j] >= '0' && input[j] <= '9') {
				j++
			}
			if j < len(input) && input[j] == '$' {
				tag := string(input[i : j+1])
				end := strings.Index(sql[j+1:], tag)
				if end < 0 {
					return nil, fmt.Errorf("DSQL compatibility: unterminated dollar quote")
				}
				i = j + 1 + end + len(tag)
			} else {
				i++
			}
			if len(prefix) < 5 {
				prefix = append(prefix, "quoted")
			}
		case sqlWord(input[i]):
			for i < len(input) && sqlWord(input[i]) {
				i++
			}
			word := strings.ToUpper(string(input[start:i]))
			if word == "ASYNC" && (strings.Join(prefix, " ") == "CREATE INDEX" || strings.Join(prefix, " ") == "CREATE UNIQUE INDEX" || strings.Join(prefix, " ") == "ALTER TABLE") {
				for j := start; j < i; j++ {
					out[j] = ' '
				}
			}
			if len(prefix) < 5 {
				prefix = append(prefix, word)
			}
		case input[i] == ';':
			prefix = nil
			i++
		case input[i] == ' ' || input[i] == '\t' || input[i] == '\n' || input[i] == '\r' || input[i] == '\f':
			i++
		default:
			if len(prefix) < 5 {
				prefix = append(prefix, "punctuation")
			}
			i++
		}
	}
	return out, nil
}

func sqlWord(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b >= 128
}
