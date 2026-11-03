package get

import (
	"bytes"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/testutil"
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
			name:    "unsupportedSource",
			source:  "keyvaluestore/dba",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			objects := []client.Object{}
			for i, ref := range tt.backups {
				objects = append(objects, testutil.DatabaseBackup("backup-"+string(rune('a'+i)), testutil.DefaultProject, ref))
			}

			apiClient := testutil.SetupClient(t, testutil.WithObjects(objects...))

			buf := &bytes.Buffer{}
			cmd := databaseBackupCmd{}
			if tt.source != "" {
				cmd.Source = testutil.TypedReference(t, tt.source)
			}
			err := cmd.Run(t.Context(), apiClient, NewTestCmd(t, buf, full))
			if tt.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
			for _, substr := range tt.wantContain {
				is.Contains(buf.String(), substr)
			}
			is.Equal(tt.wantLines, testutil.CountLines(buf.String()))
		})
	}
}

func TestDatabaseBackupBucketAccess(t *testing.T) {
	t.Parallel()

	db := testutil.PostgresDatabase("mydb", testutil.DefaultProject, "nine-es34")
	sourceRef := database.Ref(storage.PostgresDatabaseKind, db.Name)

	backup := testutil.DatabaseBackup("backup", testutil.DefaultProject, sourceRef)
	backup.Spec.ForProvider.Bucket = meta.LocalReference{Name: "backup-bucket"}
	backup.Status.AtProvider.Path = "dumps/backup.dump"

	schedule := &storage.DatabaseBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "mydb-schedule", Namespace: testutil.DefaultProject},
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
		ObjectMeta: metav1.ObjectMeta{Name: "backup-bucket", Namespace: testutil.DefaultProject},
		Status: storage.BucketStatus{
			AtProvider: storage.BucketObservation{Endpoint: "objects.nine.ch"},
		},
	}

	user := &storage.BucketUser{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-bucket", Namespace: testutil.DefaultProject},
		Spec: storage.BucketUserSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "backup-bucket",
					Namespace: testutil.DefaultProject,
				},
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-bucket", Namespace: testutil.DefaultProject},
		Data: map[string][]byte{
			storage.BucketUserCredentialAccessKey: []byte("access"),
			storage.BucketUserCredentialSecretKey: []byte("secret"),
		},
	}

	apiClient := testutil.SetupClient(t,
		testutil.WithObjects(db, backup, schedule, bucket, user, secret),
		testutil.WithNameIndexFor(&storage.DatabaseBackup{}),
		testutil.WithNameIndexFor(&storage.PostgresDatabase{}),
	)

	t.Run("backupURL", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		cmd := databaseBackupCmd{PrintURL: true}
		cmd.Name = backup.Name
		is.NoError(cmd.Run(t.Context(), apiClient, NewTestCmd(t, buf, full)))
		is.Equal("https://objects.nine.ch/backup-bucket/dumps/backup.dump\n", buf.String())
	})

	t.Run("backupCredentials", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		cmd := databaseBackupCmd{PrintCredentials: true}
		cmd.Name = backup.Name
		is.NoError(cmd.Run(t.Context(), apiClient, NewTestCmd(t, buf, full)))
		is.Contains(buf.String(), "access")
		is.Contains(buf.String(), "secret")
	})

	t.Run("databaseBackupBucket", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		dbCmd := postgresDatabaseCmd{DatabaseCmd{PrintBackupBucket: true, ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: db.Name}}}}
		is.NoError(dbCmd.Run(t.Context(), apiClient, NewTestCmd(t, buf, full)))
		is.Equal("https://objects.nine.ch/backup-bucket\n", buf.String())
	})

	t.Run("databaseBackupCredentials", func(t *testing.T) {
		is := require.New(t)
		buf := &bytes.Buffer{}

		dbCmd := postgresDatabaseCmd{DatabaseCmd{PrintBackupCredentials: true, ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: db.Name}}}}
		is.NoError(dbCmd.Run(t.Context(), apiClient, NewTestCmd(t, buf, full)))
		is.Contains(buf.String(), "access")
	})

	t.Run("backupNotFinished", func(t *testing.T) {
		running := testutil.DatabaseBackup("running", testutil.DefaultProject, sourceRef)
		running.Spec.ForProvider.Bucket = meta.LocalReference{Name: "backup-bucket"}
		runningClient := testutil.SetupClient(t,
			testutil.WithObjects(running, bucket.DeepCopy()),
			testutil.WithNameIndexFor(&storage.DatabaseBackup{}),
		)

		cmd := databaseBackupCmd{PrintURL: true}
		cmd.Name = running.Name
		require.ErrorContains(t, cmd.Run(t.Context(), runningClient, NewTestCmd(t, &bytes.Buffer{}, full)), "no path yet")
	})

	t.Run("bucketNotProvisioned", func(t *testing.T) {
		pending := bucket.DeepCopy()
		pending.Status.AtProvider.Endpoint = ""
		pendingClient := testutil.SetupClient(t,
			testutil.WithObjects(backup.DeepCopy(), pending),
			testutil.WithNameIndexFor(&storage.DatabaseBackup{}),
		)

		cmd := databaseBackupCmd{PrintURL: true}
		cmd.Name = backup.Name
		require.ErrorContains(t, cmd.Run(t.Context(), pendingClient, NewTestCmd(t, &bytes.Buffer{}, full)), "not provisioned yet")
	})

	t.Run("otherProject", func(t *testing.T) {
		const project = "other-project"
		otherBackup := backup.DeepCopy()
		otherBackup.Namespace = project
		otherBucket := bucket.DeepCopy()
		otherBucket.Namespace = project
		otherBucket.Status.AtProvider.Endpoint = "https://other.nine.ch"
		otherClient := testutil.SetupClient(t,
			testutil.WithProjects(project),
			testutil.WithObjects(otherBackup, otherBucket),
			testutil.WithNameIndexFor(&storage.DatabaseBackup{}),
			testutil.WithKubeconfig(),
		)
		buf := &bytes.Buffer{}

		cmd := databaseBackupCmd{PrintURL: true}
		cmd.Name = backup.Name
		require.NoError(t, cmd.Run(t.Context(), otherClient, NewTestCmd(t, buf, full, WithAllProjects())))
		require.Equal(t, "https://other.nine.ch/backup-bucket/dumps/backup.dump\n", buf.String())
	})

	t.Run("noBackupSchedule", func(t *testing.T) {
		is := require.New(t)
		other := testutil.PostgresDatabase("other", testutil.DefaultProject, "nine-es34")
		noScheduleClient := testutil.SetupClient(t, testutil.WithObjects(other), testutil.WithNameIndexFor(&storage.PostgresDatabase{}))

		dbCmd := postgresDatabaseCmd{DatabaseCmd{PrintBackupBucket: true, ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: other.Name}}}}
		is.ErrorContains(dbCmd.Run(t.Context(), noScheduleClient, NewTestCmd(t, &bytes.Buffer{}, full)), "no backup schedule")
	})
}

func TestExpiresIn(t *testing.T) {
	t.Parallel()

	require.Equal(t, noneText, expiresIn(time.Time{}))
	require.Equal(t, "expired", expiresIn(time.Now().Add(-time.Hour)))
	require.Equal(t, "2d", expiresIn(time.Now().Add(48*time.Hour+time.Minute)))
}

func TestDatabaseBackupExpires(t *testing.T) {
	t.Parallel()

	// the provider records the expiration in the status only
	backup := testutil.DatabaseBackup("backup", testutil.DefaultProject, database.Ref(storage.PostgresDatabaseKind, "mydb"))
	backup.Status.AtProvider.Expiration = metav1.NewTime(time.Now().Add(48*time.Hour + time.Minute))
	apiClient := testutil.SetupClient(t, testutil.WithObjects(backup))

	buf := &bytes.Buffer{}
	cmd := databaseBackupCmd{}
	require.NoError(t, cmd.Run(t.Context(), apiClient, NewTestCmd(t, buf, full)))
	require.Regexp(t, `backup\s+postgresdatabase/mydb\s+<none>\s+<none>\s+2d\s`, buf.String())
}
