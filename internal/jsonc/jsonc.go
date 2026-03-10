// Package jsonc provides utilities for parsing JSONC (JSON with Comments).
// VS Code, Cursor, and other editors use JSONC for their settings files.
package jsonc

import (
	"regexp"
	"strings"
)

// StripComments removes comments and trailing commas from JSONC to produce valid JSON.
// Handles:
//   - Single-line comments: // comment
//   - Multi-line comments: /* comment */
//   - Trailing commas before } or ]
func StripComments(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	inMultiLineComment := false

	for _, line := range lines {
		// Handle multi-line comments
		if inMultiLineComment {
			if idx := strings.Index(line, "*/"); idx != -1 {
				line = line[idx+2:]
				inMultiLineComment = false
			} else {
				continue
			}
		}

		// Remove multi-line comment starts
		for {
			startIdx := strings.Index(line, "/*")
			if startIdx == -1 {
				break
			}
			endIdx := strings.Index(line[startIdx:], "*/")
			if endIdx == -1 {
				line = line[:startIdx]
				inMultiLineComment = true
				break
			}
			line = line[:startIdx] + line[startIdx+endIdx+2:]
		}

		// Remove single-line comments (naive: assumes // not in string)
		// Find // that's not inside a string
		inString := false
		escaped := false
		for i, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if !inString && i < len(line)-1 && line[i:i+2] == "//" {
				line = line[:i]
				break
			}
		}

		result = append(result, line)
	}

	joined := strings.Join(result, "\n")

	// Remove trailing commas before } or ]
	// Match: comma followed by optional whitespace/newlines followed by } or ]
	trailingComma := regexp.MustCompile(`,(\s*)([\]}])`)
	joined = trailingComma.ReplaceAllString(joined, "$1$2")

	return joined
}
