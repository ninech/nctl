package delete

import (
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestApplication(t *testing.T) {
	ctx := context.Background()
	project := "evilcorp"
	for name, testCase := range map[string]struct {
		testObjects    testObjectList
		name           string
		errorExpected  bool
		errorCheck     func(err error) bool
		deletedSecrets []corev1.Secret
	}{
		"application-without-git-auth-secret": {
			testObjects:   toTestObj(dummyApp("dev", project)),
			name:          "dev",
			errorExpected: false,
		},
		"application-does-not-exist": {
			name:          "dev",
			errorExpected: true,
			errorCheck: func(err error) bool {
				return errors.IsNotFound(err)
			},
		},
		"application-with-git-auth-secret": {
			testObjects: func() []testObject {
				app := dummyApp("dev", project)
				secret := gitSecretFor(app)
				return toTestObj(app, secret)
			}(),
			name:          "dev",
			errorExpected: false,
		},
		"application-with-referenced-git-auth-secret": {
			testObjects: func() []testObject {
				appOne := dummyApp("dev", project)
				appTwo := dummyApp("test", project)
				secret := gitSecretFor(appOne)
				appTwo.Spec.ForProvider.Git.Auth = &apps.GitAuth{
					FromSecret: &meta.LocalReference{
						Name: secret.Name,
					},
				}
				return toTestObj(
					appOne,
					noDeletionExpected(appTwo),
					noDeletionExpected(secret),
				)
			}(),
			name:          "dev",
			errorExpected: false,
		},
		// here we have 2 secrets. One secret created by nctl which is
		// not in use and another non nctl managed secret which
		// is used by the application.
		"application-with-non-nctl-secret": {
			testObjects: func() []testObject {
				appOne := dummyApp("dev", project)
				nctlSecret := gitSecretFor(appOne)

				customSecret := nctlSecret.DeepCopy()
				customSecret.Name = "custom"
				delete(customSecret.Annotations, util.ManagedByAnnotation)
				appOne.Spec.ForProvider.Git.Auth = &apps.GitAuth{
					FromSecret: &meta.LocalReference{
						Name: customSecret.Name,
					},
				}
				return toTestObj(
					appOne,
					nctlSecret,
					noDeletionExpected(customSecret),
				)
			}(),
			name:          "dev",
			errorExpected: false,
		},
		// a secret which has the same name as the application, but no
		// nctl annotation will not be deleted
		"application-git-auth-secret-no-annotation": {
			testObjects: func() []testObject {
				appOne := dummyApp("dev", project)
				nctlSecret := gitSecretFor(appOne)
				delete(nctlSecret.Annotations, util.ManagedByAnnotation)
				return toTestObj(
					appOne,
					noDeletionExpected(nctlSecret),
				)
			}(),
			name:          "dev",
			errorExpected: false,
		},
		"application-static-egress": {
			testObjects: func() []testObject {
				appOne := dummyApp("dev", project)
				egressOne := &networking.StaticEgress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name-should-not-matter",
						Namespace: project,
					},
					Spec: networking.StaticEgressSpec{
						ForProvider: networking.StaticEgressParameters{
							Target: meta.LocalTypedReference{
								LocalReference: meta.LocalReference{
									Name: "dev",
								},
								GroupKind: metav1.GroupKind{
									Group: apps.Group,
									Kind:  apps.ApplicationKind,
								},
							},
						},
					},
				}
				egressTwo := egressOne.DeepCopy()
				egressTwo.Name = "second-egress"
				return toTestObj(
					appOne,
					egressOne,
					egressTwo,
				)
			}(),
			name:          "dev",
			errorExpected: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := applicationCmd{
				resourceCmd: resourceCmd{
					Force: true,
					Wait:  false,
					Name:  testCase.name,
				},
			}

			apiClient, err := test.SetupClient(
				test.WithDefaultProject(project),
				test.WithProjectsFromResources(testCase.testObjects.clientObjects()...),
				test.WithObjects(testCase.testObjects.clientObjects()...),
			)
			require.NoError(t, err)

			err = cmd.Run(ctx, apiClient)
			if testCase.errorExpected {
				require.Error(t, err)
				if testCase.errorCheck != nil {
					require.True(t, testCase.errorCheck(err))
				}
			} else {
				require.NoError(t, err)
			}

			for _, delObj := range testCase.testObjects {
				err := apiClient.Get(ctx, api.ObjectName(delObj), delObj.Object)
				if delObj.noDeletion {
					require.NoError(t, err)
				} else {
					require.True(t, errors.IsNotFound(err))
				}
			}
		})
	}
}

func dummyApp(name, namespace string) *apps.Application {
	return &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: apps.SchemeGroupVersion.String(),
			Kind:       apps.ApplicationKind,
		},
		Spec: apps.ApplicationSpec{},
	}
}

func gitSecretFor(app *apps.Application) *corev1.Secret {
	s := util.GitAuth{}.Secret(app)
	s.TypeMeta = metav1.TypeMeta{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Secret",
	}
	return s
}

type testObject struct {
	client.Object
	noDeletion bool
}

type testObjectList []testObject

func (l testObjectList) clientObjects() (result []client.Object) {
	for _, item := range l {
		result = append(result, item.Object)
	}
	return result
}

func toTestObj(objs ...client.Object) testObjectList {
	var result []testObject
	for _, o := range objs {
		if testObj, is := o.(testObject); is {
			result = append(result, testObj)
			continue
		}
		result = append(result, testObject{Object: o})
	}
	return result
}

func noDeletionExpected(obj client.Object) testObject {
	return testObject{
		Object:     obj,
		noDeletion: true,
	}
}
