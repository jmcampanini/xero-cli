// Package render writes terminal-aware human tables and compact JSON payloads.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"golang.org/x/term"
)

// Terminal reports whether the writer is a terminal file.
func Terminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// JSON writes one compact JSON value followed by a newline.
func JSON(w io.Writer, value any) error { return json.NewEncoder(w).Encode(value) }

// Table aligns rows and optionally colors the header for human stdout.
func Table(w io.Writer, header []string, rows [][]string, color string) error {
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
	if len(header) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(header, "\t")); err != nil {
			return err
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			row[i] = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ", "\x1b", "").Replace(cell)
		}
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	output := buf.String()
	useColor := color == "always" || color == "auto" && Terminal(w) && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
	if useColor && len(header) > 0 {
		first, rest, _ := strings.Cut(output, "\n")
		output = "\x1b[1m" + first + "\x1b[0m\n" + rest
	}
	_, err := io.WriteString(w, output)
	return err
}
