package get

import (
	"bytes"
	"os"
	"strings"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	management "github.com/ninech/apis/management/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAllContent(t *testing.T) {
	t.Parallel()
	organization := "evilcorp"

	for name, testCase := range map[string]struct {
		projects             []client.Object
		objects              []client.Object
		projectName          string
		outputFormat         outputFormat
		allProjects          bool
		includeNineResources bool
		kinds                []string
		output               string
		errorExpected        bool
		errorContains        []string
	}{
		"all resources from one project, full format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			objects:      []client.Object{testApplication("banana", "dev"), testRelease("pear", "dev")},
			outputFormat: full,
			projectName:  "dev",
			output: `PROJECT  NAME    KIND         GROUP
dev      banana  Application  apps.nine.ch
dev      pear    Release      apps.nine.ch
`,
		},
		"all resources from one project, no header": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			objects:      []client.Object{testApplication("banana", "dev"), testRelease("pear", "dev")},
			outputFormat: noHeader,
			projectName:  "dev",
			output: `dev  banana  Application  apps.nine.ch
dev  pear    Release      apps.nine.ch
`,
		},
		"all resources from one project, yaml format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			objects:      []client.Object{testApplication("banana", "dev"), testRelease("pear", "dev")},
			outputFormat: yamlOut,
			projectName:  "dev",
			output:       "apiVersion: apps.nine.ch/v1alpha1\nkind: Application\nmetadata:\n  name: banana\n  namespace: dev\nspec:\n  forProvider:\n    buildEnv: null\n    config:\n      env: null\n      port: null\n      replicas: null\n      size: \"\"\n    dockerfileBuild:\n      enabled: false\n    git:\n      revision: \"\"\n      subPath: \"\"\n      url: \"\"\n    paused: false\nstatus:\n  atProvider:\n    defaultURLs: null\n---\napiVersion: apps.nine.ch/v1alpha1\ncreationTimestampNano: 0\nkind: Release\nmetadata:\n  name: pear\n  namespace: dev\nspec:\n  forProvider:\n    build:\n      name: \"\"\n    configuration:\n      size:\n        origin: \"\"\n        value: \"\"\n    defaultHosts: null\n    healthProbeConfiguration: null\n    image: {}\n    paused: false\nstatus:\n  atProvider:\n    owning: false\n",
		},
		"all resources from one project, json format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			objects:      []client.Object{testApplication("banana", "dev"), testRelease("pear", "dev")},
			outputFormat: jsonOut,
			projectName:  "dev",
			output: `[
  {
    "apiVersion": "apps.nine.ch/v1alpha1",
    "kind": "Application",
    "metadata": {
      "name": "banana",
      "namespace": "dev"
    },
    "spec": {
      "forProvider": {
        "buildEnv": null,
        "config": {
          "env": null,
          "port": null,
          "replicas": null,
          "size": ""
        },
        "dockerfileBuild": {
          "enabled": false
        },
        "git": {
          "revision": "",
          "subPath": "",
          "url": ""
        },
        "paused": false
      }
    },
    "status": {
      "atProvider": {
        "defaultURLs": null
      }
    }
  },
  {
    "apiVersion": "apps.nine.ch/v1alpha1",
    "creationTimestampNano": 0,
    "kind": "Release",
    "metadata": {
      "name": "pear",
      "namespace": "dev"
    },
    "spec": {
      "forProvider": {
        "build": {
          "name": ""
        },
        "configuration": {
          "size": {
            "origin": "",
            "value": ""
          }
        },
        "defaultHosts": null,
        "healthProbeConfiguration": null,
        "image": {},
        "paused": false
      }
    },
    "status": {
      "atProvider": {
        "owning": false
      }
    }
  }
]
`,
		},
		"all projects, full format": {
			projects: test.Projects(organization, "dev", "staging", "prod"),
			objects: []client.Object{
				testApplication("banana", "dev"), testRelease("pear", "dev"),
				testApplication("apple", "staging"), testRelease("melon", "staging"),
				testCluster("orange", "prod"),
			},
			outputFormat: full,
			allProjects:  true,
			output: `PROJECT  NAME    KIND               GROUP
dev      banana  Application        apps.nine.ch
dev      pear    Release            apps.nine.ch
prod     orange  KubernetesCluster  infrastructure.nine.ch
staging  apple   Application        apps.nine.ch
staging  melon   Release            apps.nine.ch
`,
		},
		"all projects, no headers format": {
			projects: test.Projects(organization, "dev", "staging", "prod"),
			objects: []client.Object{
				testApplication("banana", "dev"), testRelease("pear", "dev"),
				testApplication("apple", "staging"), testRelease("melon", "staging"),
				testCluster("orange", "prod"),
			},
			outputFormat: noHeader,
			allProjects:  true,
			output: `dev      banana  Application        apps.nine.ch
dev      pear    Release            apps.nine.ch
prod     orange  KubernetesCluster  infrastructure.nine.ch
staging  apple   Application        apps.nine.ch
staging  melon   Release            apps.nine.ch
`,
		},
		"empty resources of a specific project, full format": {
			projects:      test.Projects(organization, "dev"),
			objects:       []client.Object{},
			outputFormat:  full,
			projectName:   "dev",
			errorExpected: true,
			errorContains: []string{`no "Resources" found`, `Project: dev`, "get resources --all-projects"},
		},
		"empty resources of all projects, full format": {
			projects:      test.Projects(organization, "dev", "staging"),
			objects:       []client.Object{},
			outputFormat:  full,
			allProjects:   true,
			errorExpected: true,
			errorContains: []string{`no "Resources" found`},
		},
		"filter nine resources, no headers format": {
			projects: test.Projects(organization, "dev", "staging", "prod"),
			objects: []client.Object{
				testApplication("banana", "dev"), testRelease("pear", "dev"),
				testApplication("apple", "staging"), testRelease("melon", "staging"), testRelease("cherry", "staging"),
				testCluster("orange", "prod"),
				func() *apps.Application {
					nineApp := testApplication("kiwi", "dev")
					nineApp.Labels = map[string]string{
						meta.NineOwnedLabelKey: meta.NineOwnedLabelValue,
					}
					return nineApp
				}(),
			},
			outputFormat: noHeader,
			allProjects:  true,
			output: `dev      banana  Application        apps.nine.ch
dev      pear    Release            apps.nine.ch
prod     orange  KubernetesCluster  infrastructure.nine.ch
staging  apple   Application        apps.nine.ch
staging  cherry  Release            apps.nine.ch
staging  melon   Release            apps.nine.ch
`,
		},
		"include nine resources, no headers format": {
			projects: test.Projects(organization, "dev", "staging", "prod"),
			objects: []client.Object{
				testApplication("banana", "dev"), testRelease("pear", "dev"),
				testApplication("apple", "staging"), testRelease("melon", "staging"), testRelease("cherry", "staging"),
				testCluster("orange", "prod"),
				func() *apps.Application {
					nineApp := testApplication("kiwi", "dev")
					nineApp.Labels = map[string]string{
						meta.NineOwnedLabelKey: meta.NineOwnedLabelValue,
					}
					return nineApp
				}(),
			},
			outputFormat:         noHeader,
			allProjects:          true,
			includeNineResources: true,
			output: `dev      banana  Application        apps.nine.ch
dev      kiwi    Application        apps.nine.ch
dev      pear    Release            apps.nine.ch
prod     orange  KubernetesCluster  infrastructure.nine.ch
staging  apple   Application        apps.nine.ch
staging  cherry  Release            apps.nine.ch
staging  melon   Release            apps.nine.ch
`,
		},
		"only certain kind": {
			projects: test.Projects(organization, "dev", "staging", "prod"),
			objects: []client.Object{
				testApplication("banana", "dev"), testRelease("pear", "dev"),
				testApplication("apple", "staging"), testRelease("melon", "staging"), testRelease("cherry", "staging"),
				testCluster("orange", "prod"),
			},
			outputFormat: full,
			allProjects:  true,
			kinds:        []string{"application"},
			output: `PROJECT  NAME    KIND         GROUP
dev      banana  Application  apps.nine.ch
staging  apple   Application  apps.nine.ch
`,
		},
		"multiple certain kinds, no header format": {
			projects: test.Projects(organization, "dev", "staging", "prod"),
			objects: []client.Object{
				testApplication("banana", "dev"), testRelease("pear", "dev"),
				testApplication("apple", "staging"), testRelease("melon", "staging"), testRelease("cherry", "staging"),
				testCluster("orange", "prod"),
				testCluster("dragonfruit", "dev"),
			},
			outputFormat: noHeader,
			allProjects:  true,
			kinds:        []string{"release", "kubernetescluster"},
			output: `dev      dragonfruit  KubernetesCluster  infrastructure.nine.ch
dev      pear         Release            apps.nine.ch
prod     orange       KubernetesCluster  infrastructure.nine.ch
staging  cherry       Release            apps.nine.ch
staging  melon        Release            apps.nine.ch
`,
		},
		"not known kind leads to an error": {
			projects:      test.Projects(organization, "dev", "staging", "prod"),
			objects:       []client.Object{},
			outputFormat:  noHeader,
			allProjects:   true,
			kinds:         []string{"jackofalltrades"},
			errorExpected: true,
		},
		"excluded list kinds are not shown": {
			projects: test.Projects(organization, "dev"),
			objects: []client.Object{
				testApplication("banana", "dev"), testRelease("pear", "dev"),
				testClusterData(),
			},
			outputFormat: full,
			allProjects:  true,
			output: `PROJECT  NAME    KIND         GROUP
dev      banana  Application  apps.nine.ch
dev      pear    Release      apps.nine.ch
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)
			testCase := testCase

			outputBuffer := &bytes.Buffer{}
			get := NewTestCmd(outputBuffer, testCase.outputFormat)
			get.AllProjects = testCase.allProjects

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&management.Project{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithObjects(append(testCase.projects, testCase.objects...)...).Build()

			apiClient := &api.Client{WithWatch: client, Project: testCase.projectName}
			kubeconfig, err := test.CreateTestKubeconfig(apiClient, organization)
			is.NoError(err)
			defer os.Remove(kubeconfig)

			cmd := allCmd{
				IncludeNineResources: testCase.includeNineResources,
				Kinds:                testCase.kinds,
			}

			err = cmd.Run(t.Context(), apiClient, get)
			if testCase.errorExpected {
				is.Error(err)
				for _, s := range testCase.errorContains {
					is.Contains(strings.ToLower(err.Error()), strings.ToLower(s))
				}
				return
			}
			is.NoError(err)
			is.Equal(testCase.output, outputBuffer.String())
		})
	}
}

func testApplication(name, project string) *apps.Application {
	return &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       apps.ApplicationKind,
			APIVersion: apps.SchemeGroupVersion.String(),
		},
		Spec: apps.ApplicationSpec{},
	}
}

func testRelease(name, project string) *apps.Release {
	return &apps.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       apps.ReleaseKind,
			APIVersion: apps.SchemeGroupVersion.String(),
		},
		Spec: apps.ReleaseSpec{},
	}
}

func testCluster(name, project string) *infra.KubernetesCluster {
	return &infra.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       infra.KubernetesClusterKind,
			APIVersion: infra.SchemeGroupVersion.String(),
		},
		Spec: infra.KubernetesClusterSpec{},
	}
}

func testClusterData() *infra.ClusterData {
	return &infra.ClusterData{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       infra.ClusterDataKind,
			APIVersion: infra.SchemeGroupVersion.String(),
		},
		Spec: infra.ClusterDataSpec{
			ForProvider: infra.ClusterDataParameters{
				ClusterReference: meta.Reference{
					Name: "test",
				},
			},
		},
	}
}
