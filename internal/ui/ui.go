package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Unicode marks
const (
	SuccessMark = "✓"
	ErrorMark   = "✗"
	WarnMark    = "⚠"
	ActionMark  = "→"
	MissingMark = "-"
	SkipMark    = "○"
	ItemMark    = "•"
)

var (
	successColor = color.New(color.FgGreen, color.Bold)
	errorColor   = color.New(color.FgRed, color.Bold)
	warnColor    = color.New(color.FgYellow, color.Bold)
	infoColor    = color.New(color.FgCyan)
	headerColor  = color.New(color.FgWhite, color.Bold, color.BgBlue)
	sectionColor = color.New(color.FgWhite, color.Bold)
	skipColor    = color.New(color.FgHiBlack)
)

// Header prints a prominent header line.
func Header(msg string) {
	headerColor.Printf(" %s ", msg)
	fmt.Println()
}

// SectionHeader prints a section title.
func SectionHeader(msg string) {
	sectionColor.Printf("── %s ──\n", msg)
}

// Success prints a green success message.
func Success(msg string) {
	successColor.Printf("%s ", SuccessMark)
	fmt.Println(msg)
}

// Error prints a red error message.
func Error(msg string) {
	errorColor.Printf("%s ", ErrorMark)
	fmt.Println(msg)
}

// Warn prints a yellow warning message.
func Warn(msg string) {
	warnColor.Printf("%s ", WarnMark)
	fmt.Println(msg)
}

// Info prints a cyan informational message.
func Info(msg string) {
	infoColor.Printf("  %s\n", msg)
}

// Action prints a cyan action message with an arrow.
func Action(msg string) {
	infoColor.Printf("%s %s\n", ActionMark, msg)
}

// Skip prints a grey skip message.
func Skip(msg string) {
	skipColor.Printf("%s %s\n", SkipMark, msg)
}

// Item prints a bullet point.
func Item(msg string) {
	fmt.Printf("  %s %s\n", ItemMark, msg)
}

// Table prints a simple text table from rows of strings.
// First row is treated as a header.
func Table(rows [][]string) {
	if len(rows) == 0 {
		return
	}
	// Compute column widths
	cols := len(rows[0])
	widths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if i < cols {
				l := visibleLen(cell)
				if l > widths[i] {
					widths[i] = l
				}
			}
		}
	}

	// Print header
	printRow(rows[0], widths, true)
	// Print separator
	sep := ""
	for i, w := range widths {
		if i > 0 {
			sep += "─┼─"
		}
		sep += strings.Repeat("─", w)
	}
	fmt.Println(sep)
	// Print data rows
	for _, row := range rows[1:] {
		printRow(row, widths, false)
	}
}

func printRow(row []string, widths []int, header bool) {
	line := ""
	for i, cell := range row {
		if i > 0 {
			line += " │ "
		}
		padding := 0
		if i < len(widths) {
			padding = widths[i] - visibleLen(cell)
		}
		if padding < 0 {
			padding = 0
		}
		if header {
			line += sectionColor.Sprint(cell) + strings.Repeat(" ", padding)
		} else {
			line += cell + strings.Repeat(" ", padding)
		}
	}
	fmt.Println(line)
}

// visibleLen returns the approximate visible length of a string
// (ignores ANSI escape sequences by a naive approximation).
func visibleLen(s string) int {
	count := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		count++
	}
	return count
}
