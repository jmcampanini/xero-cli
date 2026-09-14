package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestColorAndMachineOutput(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")
	for _, mode := range []string{"auto", "always", "never"} {
		t.Run(mode, func(t *testing.T) {
			var out bytes.Buffer
			if err := Table(&out, []string{"NAME"}, [][]string{{"Example"}}, mode); err != nil {
				t.Fatal(err)
			}
			if colored := strings.Contains(out.String(), "\x1b"); colored != (mode == "always") {
				t.Errorf("Table color %s = %q", mode, out.String())
			}
		})
	}
	var out bytes.Buffer
	if err := JSON(&out, map[string]string{"name": "Example"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{\"name\":\"Example\"}\n" {
		t.Errorf("JSON = %q", out.String())
	}
}
