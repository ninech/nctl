// as we need the test.CreateTestKubeconfig function and to not cause a cycle,
// we use the auth_test package
package auth_test

import (
	"bytes"
	"testing"

	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestWhoAmICmd_Run(t *testing.T) {
	t.Parallel()

	expected := auth.WhoAmIOutput{
		Account:      "jrocket@example.com",
		Organization: test.DefaultProject,
		Project:      test.DefaultProject,
		Orgs:         []string{"test", "bla"},
	}

	for name, testCase := range map[string]struct {
		format      string
		checkOutput func(t *testing.T, buf *bytes.Buffer)
	}{
		"default text format": {
			checkOutput: func(t *testing.T, buf *bytes.Buffer) {
				t.Helper()
				require.Contains(t, buf.String(), "Account: jrocket@example.com")
				require.Contains(t, buf.String(), "Organization: default")
				require.Contains(t, buf.String(), "Project: default")
			},
		},
		"yaml format": {
			format: "yaml",
			checkOutput: func(t *testing.T, buf *bytes.Buffer) {
				t.Helper()
				ref := &bytes.Buffer{}
				require.NoError(t, format.PrettyPrintObject(expected, format.PrintOpts{Out: ref, Format: format.OutputFormatTypeYAML}))
				require.Equal(t, ref.String(), buf.String())
			},
		},
		"json format": {
			format: "json",
			checkOutput: func(t *testing.T, buf *bytes.Buffer) {
				t.Helper()
				ref := &bytes.Buffer{}
				require.NoError(t, format.PrettyPrintObject(expected, format.PrintOpts{Out: ref, Format: format.OutputFormatTypeJSON}))
				require.Equal(t, ref.String(), buf.String())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			apiClient := test.SetupClient(t, test.WithKubeconfig())
			cmd := &auth.WhoAmICmd{
				Writer:    format.NewWriter(buf),
				IssuerURL: "https://auth.nine.ch/auth/realms/pub",
				ClientID:  "nineapis.ch-f178254",
				Format:    testCase.format,
			}
			require.NoError(t, cmd.Run(t.Context(), apiClient))
			testCase.checkOutput(t, buf)
		})
	}
}
