package get

import (
	"bytes"
	"testing"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDatabaseBackup(t *testing.T) {
	t.Parallel()

	dbA := meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: "dba"},
		GroupKind:      metav1.GroupKind{Group: storage.Group, Kind: storage.PostgresDatabaseKind},
	}
	dbB := meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: "dbb"},
		GroupKind:      metav1.GroupKind{Group: storage.Group, Kind: storage.PostgresDatabaseKind},
	}

	tests := []struct {
		name        string
		backups     []meta.LocalTypedReference
		source      string
		wantContain []string
		wantLines   int
		wantErr     bool
	}{
		{
			name:        "listAll",
			backups:     []meta.LocalTypedReference{dbA, dbB},
			wantContain: []string{"backup-a", "backup-b"},
			wantLines:   3,
		},
		{
			name:        "filterBySource",
			backups:     []meta.LocalTypedReference{dbA, dbB},
			source:      "postgresdatabase/dba",
			wantContain: []string{"backup-a"},
			wantLines:   2,
		},
		{
			name:    "invalidSource",
			source:  "dba",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			objects := []client.Object{}
			for i, ref := range tt.backups {
				objects = append(objects, test.DatabaseBackup("backup-"+string(rune('a'+i)), test.DefaultProject, ref))
			}

			apiClient := test.SetupClient(t, test.WithObjects(objects...))

			buf := &bytes.Buffer{}
			cmd := databaseBackupCmd{Source: tt.source}
			err := cmd.Run(t.Context(), apiClient, NewTestCmd(buf, full))
			if tt.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
			for _, substr := range tt.wantContain {
				is.Contains(buf.String(), substr)
			}
			is.Equal(tt.wantLines, test.CountLines(buf.String()))
		})
	}
}

func TestDatabaseBackupBucketAccess(t *testing.T) {
	t.Parallel()

	db := test.PostgresDatabase("mydb", test.DefaultProject, "nine-es34")
	sourceRef := database.Ref(storage.PostgresDatabaseKind, db.Name)

	backup := test.DatabaseBackup("backup", test.DefaultProject, sourceRef)
	backup.Spec.ForProvider.Bucket = meta.LocalReference{Name: "backup-bucket"}
	backup.Status.AtProvider.Path = "dumps/backup.dump"

	schedule := &storage.DatabaseBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "mydb-schedule", Namespace: test.DefaultProject},
		Spec: storage.DatabaseBackupScheduleSpec{
			ForProvider: storage.DatabaseBackupScheduleParameters{Source: sourceRef},
		},
		Status: storage.DatabaseBackupScheduleStatus{
			AtProvider: storage.DatabaseBackupScheduleObservation{
				TargetBucket: meta.LocalReference{Name: "backup-bucket"},
			},
		},
	}

	bucket := &storage.Bucket{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-bucket", Namespace: test.DefaultProject},
		Status: storage.BucketStatus{
			AtProvider: storage.BucketObservation{Endpoint: "https://objects.nine.ch"},
		},
	}

	user := &storage.BucketUser{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-bucket", Namespace: test.DefaultProject},
		Spec: storage.BucketUserSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "backup-bucket",
					Namespace: test.DefaultProject,
				},
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-bucket", Namespace: test.DefaultProject},
		Data: map[string][]byte{
			storage.BucketUserCredentialAccessKey: []byte("access"),
			storage.BucketUserCredentialSecretKey: []byte("secret"),
		},
	}

	apiClient := test.SetupClient(t,
		test.WithObjects(db, backup, schedule, bucket, user, secret),
		test.WithNameIndexFor(&storage.DatabaseBackup{}),
		test.WithNameIndexFor(&storage.PostgresDatabase{}),
	)

	t.Run("backupURL", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		cmd := databaseBackupCmd{PrintURL: true}
		cmd.Name = backup.Name
		is.NoError(cmd.Run(t.Context(), apiClient, NewTestCmd(buf, full)))
		is.Equal("https://objects.nine.ch/backup-bucket/dumps/backup.dump\n", buf.String())
	})

	t.Run("backupCredentials", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		cmd := databaseBackupCmd{PrintCredentials: true}
		cmd.Name = backup.Name
		is.NoError(cmd.Run(t.Context(), apiClient, NewTestCmd(buf, full)))
		is.Contains(buf.String(), "access")
		is.Contains(buf.String(), "secret")
	})

	t.Run("databaseBackupBucket", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		dbCmd := postgresDatabaseCmd{databaseCmd{PrintBackupBucket: true, resourceCmd: resourceCmd{Name: db.Name}}}
		is.NoError(dbCmd.Run(t.Context(), apiClient, NewTestCmd(buf, full)))
		is.Equal("https://objects.nine.ch/backup-bucket\n", buf.String())
	})

	t.Run("databaseBackupCredentials", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		dbCmd := postgresDatabaseCmd{databaseCmd{PrintBackupCredentials: true, resourceCmd: resourceCmd{Name: db.Name}}}
		is.NoError(dbCmd.Run(t.Context(), apiClient, NewTestCmd(buf, full)))
		is.Contains(buf.String(), "access")
	})

	t.Run("noBackupSchedule", func(t *testing.T) {
		is := require.New(t)
		other := test.PostgresDatabase("other", test.DefaultProject, "nine-es34")
		noScheduleClient := test.SetupClient(t, test.WithObjects(other), test.WithNameIndexFor(&storage.PostgresDatabase{}))

		dbCmd := postgresDatabaseCmd{databaseCmd{PrintBackupBucket: true, resourceCmd: resourceCmd{Name: other.Name}}}
		is.ErrorContains(dbCmd.Run(t.Context(), noScheduleClient, NewTestCmd(&bytes.Buffer{}, full)), "no backup schedule")
	})
}
