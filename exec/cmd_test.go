package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/internal/format"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// capturingCmd records the CLI name and args passed to runCommand.
type capturingCmd struct {
	name string
	args []string
}

// testSecret creates a corev1.Secret with a single username→password entry.
func testSecret(name, namespace, user, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			user: []byte(password),
		},
	}
}

// testDatabaseCmd returns a capturingCmd and a databaseCmd wired with no-op
// writer/reader and test-friendly function fields.
// When cidrs is non-nil those CIDRs are used; when nil the IP detection is
// triggered only for instance resources (which is safe to use in tests if the
// connector returns nil from AllowedCIDRs).
func testDatabaseCmd(name string, cidrs *[]meta.IPv4CIDR) (*capturingCmd, serviceCmd) {
	return testDatabaseCmdConfirmed(name, cidrs, false)
}

// testDatabaseCmdConfirmed is like testDatabaseCmd but pre-seeds the reader
// with "y\n" so that confirmation prompts are auto-accepted.
func testDatabaseCmdConfirmed(name string, cidrs *[]meta.IPv4CIDR, confirmed bool) (*capturingCmd, serviceCmd) {
	var reader io.Reader = &bytes.Buffer{}
	if confirmed {
		reader = strings.NewReader("y\n")
	}
	cap := &capturingCmd{}
	cmd := serviceCmd{
		resourceCmd:  resourceCmd{Name: name},
		Writer:       format.NewWriter(&bytes.Buffer{}),
		Reader:       format.NewReader(reader),
		AllowedCidrs: cidrs,
		WaitTimeout:  0,
		runCommand: func(_ context.Context, n string, args []string) error {
			cap.name = n
			cap.args = args
			return nil
		},
		lookPath: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		waitForConnectivity: func(_ context.Context, _ format.Writer, _ string, _ time.Duration) error {
			return nil
		},
		openTTYForConfirm: func() (io.ReadCloser, error) {
			return nil, fmt.Errorf("no tty in tests")
		},
	}
	return cap, cmd
}
