package format

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// WriteBase64Decoded decodes the base64 encoded string s and writes the decoded content,
// followed by a newline, to out.
// Surrounding whitespace of both the encoded and the decoded content is dropped.
// An empty s writes nothing.
func WriteBase64Decoded(out io.Writer, s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("unable to decode base64: %w", err)
	}

	_, err = fmt.Fprintln(out, string(bytes.TrimSpace(decoded)))
	return err
}
