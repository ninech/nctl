package get

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestListPrint(t *testing.T) {
	tests := map[string]struct {
		out               outputFormat
		inAllProjects     bool
		existingResources []client.Object
		toCreate          []client.Object
		watch             bool
		wantContain       []string
		wantLines         int
		wantErr           bool
	}{
		"watch disabled": {
			out: full,
			existingResources: []client.Object{
				test.CloudVirtualMachine("foo", test.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
			},
			toCreate:    []client.Object{test.CloudVirtualMachine("new", test.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on"))},
			wantContain: []string{"foo"},
			wantLines:   2,
		},
		"watch": {
			out: full,
			existingResources: []client.Object{
				test.CloudVirtualMachine("foo", test.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
			},
			toCreate: []client.Object{
				test.CloudVirtualMachine("new", test.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
				test.CloudVirtualMachine("new2", test.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
				test.CloudVirtualMachine("new3", "other-project", "nine-es34", infrastructure.VirtualMachinePowerState("on")),
			},
			wantContain: []string{"new", "new2"},
			wantLines:   4,
			watch:       true,
		},
		// TODO: watch currently does not support the all-projects or
		// all-namespaces flags. This test should pass once that's implemented.
		//
		// "watch all projects": {
		// 	out: full,
		// 	existingResources: []client.Object{
		// 		test.CloudVirtualMachine("foo", test.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
		// 	},
		// 	toCreate: []client.Object{
		// 		test.CloudVirtualMachine("new", test.DefaultProject, "nine-es34", infrastructure.VirtualMachinePowerState("on")),
		// 		test.CloudVirtualMachine("new2", "default-project", "nine-es34", infrastructure.VirtualMachinePowerState("on")),
		// 	},
		// 	wantContain:   []string{"foo", "new2"},
		// 	wantLines:     4,
		// 	inAllProjects: true,
		// 	watch:         true,
		// },
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			apiClient, err := test.SetupClient(
				test.WithDefaultProject(test.DefaultProject),
				test.WithProjectsFromResources(append(tc.existingResources, tc.toCreate...)...),
				test.WithObjects(tc.existingResources...),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := &Cmd{output: output{Format: tc.out, AllProjects: tc.inAllProjects, writer: buf, Watch: tc.watch}}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				if err := cmd.listPrint(ctx, apiClient, &cloudVMCmd{}); (err != nil) != tc.wantErr {
					t.Errorf("cmd.list error = %v, wantErr %v", err, tc.wantErr)
					t.Log(buf.String())
				}
				wg.Done()
			}()
			// delay the creation so watch is running
			time.Sleep(time.Millisecond * 10)
			for _, res := range tc.toCreate {
				require.NoError(t, apiClient.Create(ctx, res))
			}
			wg.Wait()
			if tc.wantErr {
				return
			}
			for _, substr := range tc.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("cmd.list did not contain %q, out = %q", tc.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tc.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tc.wantLines, test.CountLines(buf.String()))
			}
		})
	}
}
