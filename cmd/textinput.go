package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// resolveText resolves a free-form text slot shared by new, note and query.
// An inline value of "-" reads stdin to EOF; a non-empty file reads that path
// verbatim ("-" is a synonym for stdin); otherwise the inline value is used.
// Exactly one trailing newline is trimmed so heredoc, printf and file input
// are byte-identical. inlineName names the inline slot in error messages.
func resolveText(cmd *cobra.Command, inline, file, inlineName string) (string, error) {
	if inline != "" && file != "" {
		return "", fmt.Errorf("cannot use both %s and --file", inlineName)
	}

	var text string
	switch {
	case file == "-" || (file == "" && inline == "-"):
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		text = string(data)
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading file: %w", err)
		}
		text = string(data)
	default:
		text = inline
	}

	return strings.TrimSuffix(text, "\n"), nil
}
