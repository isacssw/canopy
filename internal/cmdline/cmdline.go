package cmdline

import "strings"

// Fields splits a shell-like command string into arguments.
// It supports simple single-quote, double-quote and backslash escaping.
func Fields(command string) []string {
	var (
		fields       []string
		b            strings.Builder
		inSingle     bool
		inDouble     bool
		escaped      bool
		tokenStarted bool
	)

	flush := func() {
		if !tokenStarted {
			return
		}
		fields = append(fields, b.String())
		b.Reset()
		tokenStarted = false
	}

	for _, r := range command {
		if escaped {
			b.WriteRune(r)
			tokenStarted = true
			escaped = false
			continue
		}

		switch r {
		case '\\':
			if inSingle {
				b.WriteRune(r)
			} else {
				escaped = true
			}
			tokenStarted = true
		case '\'':
			if inDouble {
				b.WriteRune(r)
			} else {
				inSingle = !inSingle
			}
			tokenStarted = true
		case '"':
			if inSingle {
				b.WriteRune(r)
			} else {
				inDouble = !inDouble
			}
			tokenStarted = true
		case ' ', '\t', '\n', '\r':
			if inSingle || inDouble {
				b.WriteRune(r)
				tokenStarted = true
				continue
			}
			flush()
		default:
			b.WriteRune(r)
			tokenStarted = true
		}
	}

	if escaped {
		b.WriteRune('\\')
	}
	flush()

	return fields
}

// Executable returns the executable token from a shell-like command string.
func Executable(command string) string {
	fields := Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
