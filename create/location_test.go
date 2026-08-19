package create

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// deniedLocation builds the error the API server returns when it denies a
// create because of its location.
func deniedLocation(available []string) *apierrors.StatusError {
	err := apierrors.NewInvalid(
		schema.GroupKind{Group: "storage.nine.ch", Kind: "MySQL"},
		"test",
		nil,
	)

	err.ErrStatus.Details.Causes = []metav1.StatusCause{
		{
			Message: fmt.Sprintf("resource in location not allowed, available locations: %v", available),
			Field:   "field validation error",
		},
		{
			// spelled out on purpose. Using meta.CauseTypeLocationRestricted
			// here would make the test pass whatever that constant is set to.
			Type:    "LocationRestricted",
			Field:   "spec.forProvider.location",
			Message: strings.Join(available, " "),
		},
	}

	return err
}

// deniedLocationWithoutCause builds the same denial as an API server that does
// not report the available locations as a status cause yet.
func deniedLocationWithoutCause(available []string) *apierrors.StatusError {
	err := deniedLocation(available)
	err.ErrStatus.Details.Causes = err.ErrStatus.Details.Causes[:1]

	return err
}

func TestAvailableLocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want []meta.LocationName
	}{
		{
			name: "nil error",
			err:  nil,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection refused"),
		},
		{
			name: "unrelated api error",
			err: apierrors.NewAlreadyExists(
				schema.GroupResource{Group: "storage.nine.ch", Resource: "mysqls"}, "test",
			),
		},
		{
			name: "denial with status cause",
			err:  deniedLocation([]string{"nine-cz42", "nine-es34"}),
			want: []meta.LocationName{meta.LocationNineCZ42, meta.LocationNineES34},
		},
		{
			name: "denial from a server without the status cause",
			err:  deniedLocationWithoutCause([]string{"nine-cz42", "nine-es34"}),
		},
		{
			name: "single location",
			err:  deniedLocation([]string{"nine-es34"}),
			want: []meta.LocationName{meta.LocationNineES34},
		},
		{
			name: "no location available",
			err:  deniedLocation([]string{}),
		},
		{
			name: "wrapped denial",
			err:  fmt.Errorf("unable to create MySQL %q: %w", "test", deniedLocation([]string{"nine-cz42"})),
			want: []meta.LocationName{meta.LocationNineCZ42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tt.want, availableLocations(tt.err)); diff != "" {
				t.Errorf("availableLocations() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestCreateLocationFallback checks the full create path, including that the
// retried resource is the one that ends up stored.
func TestCreateLocationFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		location meta.LocationName
		// denials is the number of creates denied before one is allowed
		// through.
		denials int
		// available are the locations the denial reports, defaulting to two
		// when nil.
		available []string
		want      meta.LocationName
		wantErr   bool
	}{
		{
			name:    "no location requested falls back",
			denials: 1,
			want:    meta.LocationNineCZ42,
		},
		{
			name:      "denial naming no location is not retried",
			denials:   1,
			available: []string{},
			wantErr:   true,
		},
		{
			name:     "requested location is not overridden",
			location: meta.LocationNineCZ41,
			denials:  1,
			wantErr:  true,
		},
		{
			name:     "requested location that is allowed is kept",
			location: meta.LocationNineES34,
			want:     meta.LocationNineES34,
		},
		{
			name:    "fallback is only retried once",
			denials: 2,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			available := tt.available
			if available == nil {
				available = []string{"nine-cz42", "nine-es34"}
			}

			denied := 0
			cmd := mySQLCmd{Location: tt.location}
			cmd.Name = "test-mysql"
			cmd.Wait = false
			cmd.WaitTimeout = time.Second

			apiClient := test.SetupClient(t, test.WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if denied < tt.denials {
						denied++
						return deniedLocation(available)
					}
					return c.Create(ctx, obj, opts...)
				},
			}))

			err := cmd.Run(t.Context(), apiClient)
			if (err != nil) != tt.wantErr {
				t.Fatalf("mySQLCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			created := &storage.MySQL{
				ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: apiClient.Project},
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}
			if got := created.Spec.ForProvider.Location; got != tt.want {
				t.Errorf("location = %q, want %q", got, tt.want)
			}
		})
	}
}
