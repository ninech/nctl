package create

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ParseAuthorizedKeys reads SSH public keys from r. Every key is expected on
// its own line in the SSH format defined in RFC4253, blank lines and lines
// prefixed with # are ignored.
//
// The keys are validated with [ssh.ParseAuthorizedKey] but returned as they
// were read, only stripped of surrounding whitespace, so that key comments and
// options are preserved.
func ParseAuthorizedKeys(r io.Reader) ([]string, error) {
	var keys []string

	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		key := strings.TrimSpace(scanner.Text())
		if key == "" || strings.HasPrefix(key, "#") {
			continue
		}

		if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key)); err != nil {
			return nil, fmt.Errorf("invalid SSH public key on line %d: %w", line, err)
		}

		keys = append(keys, key)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}
