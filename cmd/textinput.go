package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadText implements the free-form text contract shared by new and note:
// the text is given inline, read from stdin when inline is "-", or read from
// the file named by file. Giving both inline text and a file is an error.
// hasInline distinguishes an absent value from an empty one.
func ReadText(stdin io.Reader, inline string, hasInline bool, file string) (string, error) {
	if hasInline && file != "" {
		return "", errors.New("give text inline or with -F, not both")
	}
	switch {
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", file, err)
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	case hasInline && inline == "-":
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	return inline, nil
}
