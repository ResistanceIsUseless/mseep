package style

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Color scheme
	primaryColor   = lipgloss.Color("#00D9FF")
	successColor   = lipgloss.Color("#32CD32")
	warningColor   = lipgloss.Color("#FFA500")
	errorColor     = lipgloss.Color("#FF6B6B")
	mutedColor     = lipgloss.Color("#6C7B7F")
	borderColor    = lipgloss.Color("#3C3C3C")
	
	// Base styles
	bold   = lipgloss.NewStyle().Bold(true)
	italic = lipgloss.NewStyle().Italic(true)
	
	// Component styles
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Margin(1, 0)
	
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Margin(1, 0, 0, 0)
	
	successStyle = lipgloss.NewStyle().
		Foreground(successColor).
		Bold(true)
	
	warningStyle = lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true)
	
	errorStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)
	
	mutedStyle = lipgloss.NewStyle().
		Foreground(mutedColor)
	
	codeStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#1E1E1E")).
		Foreground(lipgloss.Color("#E0E0E0")).
		Padding(0, 1).
		Margin(0, 0, 1, 2)
	
	boxStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Margin(1, 0)
		
	listItemStyle = lipgloss.NewStyle().
		PaddingLeft(2)
)

// Title renders a prominent title
func Title(text string) string {
	return titleStyle.Render(text)
}

// Header renders a section header
func Header(text string) string {
	return headerStyle.Render(text)
}

// Success renders success text
func Success(text string) string {
	return successStyle.Render("✓ " + text)
}

// Warning renders warning text
func Warning(text string) string {
	return warningStyle.Render("⚠ " + text)
}

// Error renders error text
func Error(text string) string {
	return errorStyle.Render("✗ " + text)
}

// Muted renders muted/secondary text
func Muted(text string) string {
	return mutedStyle.Render(text)
}

// Code renders code or file paths
func Code(text string) string {
	return codeStyle.Render(text)
}

// Box renders content in a bordered box
func Box(content string) string {
	return boxStyle.Render(content)
}

// ListItem renders a list item with proper indentation
func ListItem(text string) string {
	return listItemStyle.Render("• " + text)
}

// StatusTable creates a nice status table
func StatusTable(rows [][]string, headers []string) string {
	if len(rows) == 0 {
		return Muted("No data")
	}
	
	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}
	
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}
	
	var output strings.Builder
	
	// Header
	headerRow := make([]string, len(headers))
	for i, header := range headers {
		headerRow[i] = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Width(colWidths[i]).
			Render(header)
	}
	output.WriteString(strings.Join(headerRow, " │ ") + "\n")
	
	// Separator
	separators := make([]string, len(headers))
	for i := range headers {
		separators[i] = strings.Repeat("─", colWidths[i])
	}
	output.WriteString(strings.Join(separators, "─┼─") + "\n")
	
	// Data rows
	for _, row := range rows {
		formattedRow := make([]string, len(headers))
		for i, cell := range row {
			if i < len(formattedRow) {
				// Apply color based on content
				var style lipgloss.Style
				switch {
				case strings.Contains(cell, "✓"):
					style = successStyle
				case strings.Contains(cell, "⚠"):
					style = warningStyle
				case strings.Contains(cell, "✗"):
					style = errorStyle
				default:
					style = lipgloss.NewStyle()
				}
				
				formattedRow[i] = style.Width(colWidths[i]).Render(cell)
			}
		}
		output.WriteString(strings.Join(formattedRow, " │ ") + "\n")
	}
	
	return output.String()
}

// DiffBox renders a diff in a styled box
func DiffBox(diff string) string {
	if diff == "No changes" {
		return Box(Muted("No changes"))
	}
	
	// Style diff lines
	lines := strings.Split(diff, "\n")
	var styledLines []string
	
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+"):
			styledLines = append(styledLines, successStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			styledLines = append(styledLines, errorStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			styledLines = append(styledLines, mutedStyle.Render(line))
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			styledLines = append(styledLines, headerStyle.Render(line))
		default:
			styledLines = append(styledLines, line)
		}
	}
	
	return boxStyle.Render(strings.Join(styledLines, "\n"))
}

// Separator renders a horizontal separator
func Separator(char string, width int) string {
	if width <= 0 {
		width = 60
	}
	return mutedStyle.Render(strings.Repeat(char, width))
}

// Banner creates a prominent banner message
func Banner(message string) string {
	width := len(message) + 4
	if width < 40 {
		width = 40
	}
	
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Background(lipgloss.Color("#1A1A1A")).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(primaryColor).
		Width(width).
		Align(lipgloss.Center).
		Padding(1, 2)
	
	return banner.Render(message)
}

// ProgressStep shows a step in progress
func ProgressStep(step int, total int, message string) string {
	progress := fmt.Sprintf("[%d/%d]", step, total)
	return lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render(progress) + " " + message
}