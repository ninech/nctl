package get

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// cancelAfterWatch forwards the events of a watch and calls cancel once
// the consumer received n of them. This stops a watch loop deterministically
// after it has processed all expected events, without relying on timeouts.
type cancelAfterWatch struct {
	watch.Interface
	result  chan watch.Event
	stopped chan struct{}
}

func cancelAfter(w watch.Interface, n int, cancel context.CancelFunc) watch.Interface {
	cw := &cancelAfterWatch{
		Interface: w,
		// unbuffered, so a send completes only once the consumer received it
		result:  make(chan watch.Event),
		stopped: make(chan struct{}),
	}
	go func() {
		defer close(cw.result)
		received := 0
		for ev := range w.ResultChan() {
			select {
			case cw.result <- ev:
			case <-cw.stopped:
				return
			}
			received++
			if received == n {
				cancel()
			}
		}
	}()
	return cw
}

func (w *cancelAfterWatch) ResultChan() <-chan watch.Event { return w.result }

func (w *cancelAfterWatch) Stop() {
	close(w.stopped)
	w.Interface.Stop()
}

// NewTestCmd creates a [Cmd] for testing with the given writer and output format.
// It calls BeforeApply as Kong does, so the output is written to w
// and the diagnostics to the output of t.
func NewTestCmd(t testing.TB, w io.Writer, outFormat outputFormat, opts ...func(*Cmd)) *Cmd {
	t.Helper()

	cmd := &Cmd{output: output{Format: outFormat}}
	require.NoError(t, cmd.BeforeApply(format.IOStreams{Out: w, ErrOut: t.Output()}))
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

func TestListPrint(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		out               outputFormat
		inAllProjects     bool
		existingResources []client.Object
		toCreate          []client.Object
		watch             bool
		wantEvents        int
		wantContain       []string
		wantLines         int
		wantErr           bool
	}{
		"watch disabled": {
			out: full,
			existingResources: []client.Object{
				testutil.CloudVirtualMachine("foo", testutil.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
			},
			toCreate:    []client.Object{testutil.CloudVirtualMachine("new", testutil.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on"))},
			wantContain: []string{"foo"},
			wantLines:   2,
		},
		"watch": {
			out: full,
			existingResources: []client.Object{
				testutil.CloudVirtualMachine("foo", testutil.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
			},
			toCreate: []client.Object{
				testutil.CloudVirtualMachine("new", testutil.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
				testutil.CloudVirtualMachine("new2", testutil.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
				testutil.CloudVirtualMachine("new3", "other-project", "nine-es34", infrastructure.VirtualMachinePowerState("on")),
			},
			wantContain: []string{"new", "new2"},
			wantLines:   4,
			// new3 is in another project and therefore not watched
			wantEvents: 2,
			watch:      true,
		},
		// TODO: watch currently does not support the all-projects or
		// all-namespaces flags. This test should pass once that's implemented.
		//
		// "watch all projects": {
		// 	out: full,
		// 	existingResources: []client.Object{
		// 		testutil.CloudVirtualMachine("foo", testutil.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
		// 	},
		// 	toCreate: []client.Object{
		// 		testutil.CloudVirtualMachine("new", testutil.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
		// 		testutil.CloudVirtualMachine("new2", "default-project", "nine-es34", infrastructure.VirtualMachinePowerState("on")),
		// 	},
		// 	wantContain:   []string{"foo", "new2"},
		// 	wantLines:     4,
		// 	inAllProjects: true,
		// 	watch:         true,
		// },
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			// The timeout only guards against a hanging test: a watch is
			// stopped as soon as it delivered the expected events.
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			// The fake client ignores the resource version of a watch and only
			// delivers events that happen after the watch has been
			// established, so the resources are created once it is.
			watching := make(chan struct{})
			apiClient := testutil.SetupClient(t,
				testutil.WithDefaultProject(testutil.DefaultProject),
				testutil.WithProjectsFromResources(append(tc.existingResources, tc.toCreate...)...),
				testutil.WithObjects(tc.existingResources...),
				testutil.WithKubeconfig(),
				testutil.WithInterceptorFuncs(interceptor.Funcs{
					Watch: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
						w, err := c.Watch(ctx, list, opts...)
						if err != nil {
							return nil, err
						}
						close(watching)
						return cancelAfter(w, tc.wantEvents, cancel), nil
					},
				}),
			)

			buf := &bytes.Buffer{}
			cmd := NewTestCmd(t, buf, tc.out)
			cmd.AllProjects = tc.inAllProjects
			cmd.Watch = tc.watch

			listed := make(chan error, 1)
			go func() { listed <- cmd.listPrint(ctx, apiClient, &cloudVMCmd{}) }()
			checkErr := func(err error) {
				t.Helper()
				if (err != nil) != tc.wantErr {
					t.Fatalf("cmd.list error = %v, wantErr %v, out = %q", err, tc.wantErr, buf.String())
				}
			}

			if tc.watch {
				select {
				case <-watching:
				case err := <-listed:
					t.Fatalf("cmd.list returned before watching: %v", err)
				}
			} else {
				// without watch, the resources are created after the list
				// has been printed and must not appear in the output.
				checkErr(<-listed)
			}
			for _, res := range tc.toCreate {
				is.NoError(apiClient.Create(ctx, res))
			}
			if tc.watch {
				checkErr(<-listed)
			}
			if tc.wantErr {
				return
			}
			for _, substr := range tc.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("cmd.list did not contain %q, out = %q", tc.wantContain, buf.String())
				}
			}
			if testutil.CountLines(buf.String()) != tc.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tc.wantLines, testutil.CountLines(buf.String()))
			}
		})
	}
}
