package copy

import (
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api/gitinfo"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestApplication(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		source              *apps.Application
		sourceGitAuthSecret *corev1.Secret
		staticEgress        *networking.StaticEgress
		cmd                 applicationCmd
		expectedErr         string
	}{
		"same project": {
			source: newApp("source", apps.ApplicationSpec{}),
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name:       "source",
					TargetName: "target",
				},
			},
		},
		"to different project": {
			source: newApp("source", apps.ApplicationSpec{}),
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name:          "source",
					TargetName:    "target",
					TargetProject: "project-2",
				},
			},
		},
		"hosts are not copied": {
			source: newApp("source", apps.ApplicationSpec{
				ForProvider: apps.ApplicationParameters{
					Hosts: []string{"foo.example.org"},
				},
			}),
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name:       "source",
					TargetName: "target",
				},
			},
		},
		"hosts are copied": {
			source: newApp("source", apps.ApplicationSpec{
				ForProvider: apps.ApplicationParameters{
					Hosts: []string{"foo.example.org"},
				},
			}),
			cmd: applicationCmd{
				CopyHosts: true,
				resourceCmd: resourceCmd{
					Name:       "source",
					TargetName: "target",
				},
			},
		},
		"spec fields are copied": {
			source: newApp("source", apps.ApplicationSpec{
				ForProvider: apps.ApplicationParameters{
					Language: "No",
					BuildEnv: []apps.EnvVar{{Name: "SOME_VAR", Value: "some val"}},
				},
			}),
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name:       "source",
					TargetName: "target",
				},
			},
		},
		"source does not exist": {
			source: newApp("source", apps.ApplicationSpec{}),
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name:       "does-not-exist",
					TargetName: "target",
				},
			},
			expectedErr: "unable to copy app",
		},
		"start app": {
			source: newApp("source", apps.ApplicationSpec{}),
			cmd: applicationCmd{
				Start: true,
				resourceCmd: resourceCmd{
					Name:       "source",
					TargetName: "target",
				},
			},
		},
		"git auth secret": {
			source: newApp("source", apps.ApplicationSpec{
				ForProvider: apps.ApplicationParameters{
					Git: apps.ApplicationGitConfig{
						Auth: &apps.GitAuth{
							FromSecret: &meta.LocalReference{
								Name: "source",
							},
						},
					},
				},
			}),
			sourceGitAuthSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: "default"},
				Data:       map[string][]byte{"foo": []byte("bar")},
			},
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name:       "source",
					TargetName: "target",
				},
			},
		},
		"app with static egress": {
			source: newApp("source", apps.ApplicationSpec{}),
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name:       "source",
					TargetName: "target",
				},
			},
			staticEgress: &networking.StaticEgress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source",
					Namespace: "default",
				},
				Spec: networking.StaticEgressSpec{
					ForProvider: networking.StaticEgressParameters{
						Target: meta.LocalTypedReference{
							LocalReference: meta.LocalReference{
								Name: "source",
							},
							GroupKind: metav1.GroupKind{
								Group: apps.Group,
								Kind:  apps.ApplicationKind,
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			objs := []client.Object{tc.source}
			if tc.sourceGitAuthSecret != nil {
				objs = append(objs, tc.sourceGitAuthSecret)
			}
			if tc.staticEgress != nil {
				objs = append(objs, tc.staticEgress)
			}
			apiClient := test.SetupClient(t, test.WithObjects(objs...))

			err := tc.cmd.Run(t.Context(), apiClient)
			if tc.expectedErr != "" {
				is.ErrorContains(err, tc.expectedErr)
				return
			}
			is.NoError(err)

			copiedApp := &apps.Application{}
			newName := types.NamespacedName{Name: tc.cmd.TargetName, Namespace: tc.cmd.targetNamespace(apiClient)}
			is.NoError(apiClient.Get(t.Context(), newName, copiedApp))
			// expect copied app to be paused and hosts to be empty
			tc.source.Spec.ForProvider.Paused = !tc.cmd.Start
			if !tc.cmd.CopyHosts {
				tc.source.Spec.ForProvider.Hosts = nil
			}
			is.Equal(tc.source.Spec, copiedApp.Spec)

			// check if git auth has been copied if there's a source
			if tc.sourceGitAuthSecret != nil {
				copiedSecret := &corev1.Secret{}
				newSecretName := types.NamespacedName{Name: gitinfo.AuthSecretName(copiedApp), Namespace: tc.cmd.targetNamespace(apiClient)}
				is.NoError(apiClient.Get(t.Context(), newSecretName, copiedSecret))
				is.Equal(tc.sourceGitAuthSecret.Data, copiedSecret.Data)
			}
			// check if static egress has been copied if there's a source
			if tc.staticEgress != nil {
				copiedEgress := &networking.StaticEgress{}
				newEgressName := types.NamespacedName{Name: copiedApp.Name, Namespace: tc.cmd.targetNamespace(apiClient)}
				is.NoError(apiClient.Get(t.Context(), newEgressName, copiedEgress))
				is.Equal(tc.cmd.TargetName, copiedEgress.Spec.ForProvider.Target.Name)
			}
		})
	}
}

func newApp(name string, spec apps.ApplicationSpec) *apps.Application {
	return &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: spec,
	}
}
