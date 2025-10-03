package diff

import (
	"fmt"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// GenerateUnifiedDiff creates a unified diff between two strings
func GenerateUnifiedDiff(before, after, beforeName, afterName string) string {
	if before == after {
		return "No changes"
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(before, after, false)
	
	// Convert to line-based diff for better readability
	linesBefore := strings.Split(before, "\n")
	linesAfter := strings.Split(after, "\n")
	
	// Generate patch
	patches := dmp.PatchMake(before, diffs)
	if len(patches) == 0 {
		return "No changes"
	}

	// Format as unified diff
	var output strings.Builder
	output.WriteString(fmt.Sprintf("--- %s\n", beforeName))
	output.WriteString(fmt.Sprintf("+++ %s\n", afterName))
	
	for _, patch := range patches {
		// Add context lines
		start1 := patch.Start1 + 1
		length1 := patch.Length1
		start2 := patch.Start2 + 1
		length2 := patch.Length2
		
		output.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", start1, length1, start2, length2))
		
		// Split patch text into lines and add appropriate prefixes
		patchLines := strings.Split(patch.String(), "\n")
		for i, line := range patchLines {
			if i == 0 {
				// Skip the header line that patch.String() includes
				continue
			}
			if strings.HasPrefix(line, "-") {
				output.WriteString(line + "\n")
			} else if strings.HasPrefix(line, "+") {
				output.WriteString(line + "\n")
			} else if strings.HasPrefix(line, " ") {
				output.WriteString(line + "\n")
			}
		}
	}
	
	return output.String()
}

// GenerateColorDiff creates a colored diff for terminal output
func GenerateColorDiff(before, after string) string {
	if before == after {
		return "No changes"
	}
	
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorGreen  = "\033[32m"
		colorCyan   = "\033[36m"
	)
	
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(before, after, false)
	diffs = dmp.DiffCleanupSemantic(diffs)
	
	var output strings.Builder
	output.WriteString(colorCyan + "--- before\n" + colorReset)
	output.WriteString(colorCyan + "+++ after\n" + colorReset)
	
	// Convert diffs to line-based output with colors
	var currentLine strings.Builder
	lineNum := 0
	
	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")
		
		for i, line := range lines {
			if i > 0 || currentLine.Len() > 0 {
				// We've hit a newline, output the accumulated line
				if currentLine.Len() > 0 {
					lineNum++
					output.WriteString(currentLine.String() + "\n")
					currentLine.Reset()
				}
			}
			
			if i < len(lines)-1 {
				// This is a complete line
				lineNum++
				switch diff.Type {
				case diffmatchpatch.DiffDelete:
					output.WriteString(colorRed + "-" + line + colorReset + "\n")
				case diffmatchpatch.DiffInsert:
					output.WriteString(colorGreen + "+" + line + colorReset + "\n")
				case diffmatchpatch.DiffEqual:
					// Only show a few lines of context
					output.WriteString(" " + line + "\n")
				}
			} else if line != "" {
				// This is a partial line, accumulate it
				switch diff.Type {
				case diffmatchpatch.DiffDelete:
					currentLine.WriteString(colorRed + "-" + line + colorReset)
				case diffmatchpatch.DiffInsert:
					currentLine.WriteString(colorGreen + "+" + line + colorReset)
				case diffmatchpatch.DiffEqual:
					currentLine.WriteString(" " + line)
				}
			}
		}
	}
	
	// Output any remaining partial line
	if currentLine.Len() > 0 {
		output.WriteString(currentLine.String() + "\n")
	}
	
	return output.String()
}

// SimpleDiff generates a simple side-by-side comparison
func SimpleDiff(before, after string) string {
	if before == after {
		return "No changes"
	}
	
	linesBefore := strings.Split(before, "\n")
	linesAfter := strings.Split(after, "\n")
	
	var output strings.Builder
	output.WriteString("Configuration changes:\n")
	output.WriteString("=====================\n\n")
	
	// Find the differences at line level
	maxLines := len(linesBefore)
	if len(linesAfter) > maxLines {
		maxLines = len(linesAfter)
	}
	
	hasChanges := false
	for i := 0; i < maxLines; i++ {
		var beforeLine, afterLine string
		
		if i < len(linesBefore) {
			beforeLine = linesBefore[i]
		}
		if i < len(linesAfter) {
			afterLine = linesAfter[i]
		}
		
		if beforeLine != afterLine {
			hasChanges = true
			if beforeLine != "" && afterLine == "" {
				output.WriteString(fmt.Sprintf("- %s\n", beforeLine))
			} else if beforeLine == "" && afterLine != "" {
				output.WriteString(fmt.Sprintf("+ %s\n", afterLine))
			} else if beforeLine != afterLine {
				output.WriteString(fmt.Sprintf("- %s\n", beforeLine))
				output.WriteString(fmt.Sprintf("+ %s\n", afterLine))
			}
		}
	}
	
	if !hasChanges {
		return "No changes"
	}
	
	return output.String()
}