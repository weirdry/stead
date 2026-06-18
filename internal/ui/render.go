package ui

import (
	"fmt"
	"io"
	"strings"
)

func PrintTitle(out io.Writer, title string) {
	fmt.Fprintln(out, Title(out, title))
	fmt.Fprintln(out, Rule(out, strings.Repeat("=", len(title))))
}

func PrintSection(out io.Writer, title string) {
	fmt.Fprintln(out, Section(out, title))
	fmt.Fprintln(out, Rule(out, strings.Repeat("-", len(title))))
}

func PrintKV(out io.Writer, label, value string) {
	if value == "" {
		fmt.Fprintf(out, "  %s\n", Label(out, label+":"))
		return
	}
	padded := fmt.Sprintf("%-30s", label+":")
	fmt.Fprintf(out, "  %s %s\n", Label(out, padded), value)
}

func PrintSubKV(out io.Writer, label, value string) {
	padded := fmt.Sprintf("%-28s", label+":")
	fmt.Fprintf(out, "    %s %s\n", Label(out, padded), value)
}

func StateDetail(out io.Writer, state, detail string) string {
	if detail == "" {
		return State(out, state)
	}
	return fmt.Sprintf("%s %s", State(out, state), Detail(out, "("+detail+")"))
}

func PrintListItem(out io.Writer, text string) {
	fmt.Fprintf(out, "  %s\n", text)
}

func PrintStep(out io.Writer, n int, text string) {
	fmt.Fprintf(out, "  %d. %s\n", n, text)
}
