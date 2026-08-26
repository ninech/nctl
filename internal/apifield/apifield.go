// Package apifield defines known API field defaults and acceptable values for CLI flags.
package apifield

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/posener/complete"
)

type field struct {
	// Default value used as a flag placeholder.
	Default string
	// Values accepted by the field, used for flag help text and completion.
	Values []string
}

// mysqlDatabaseCharacterSet is the default and only character set accepted for MySQL databases.
const mysqlDatabaseCharacterSet = "utf8mb4"

const (
	namespace = "apifield"
	separator = ":"
)

func qualify(name string) string {
	return namespace + separator + name
}

func unqualify(tag string) (string, bool) {
	return strings.CutPrefix(tag, namespace+separator)
}

// fields maps <resource>_<field> names to their known defaults and values.
var fields = map[string]field{
	// PostgreSQL
	"postgres_location": {
		Default: string(storage.PostgresLocationDefault),
		Values:  stringSlice(storage.PostgresLocationOptions),
	},
	"postgres_machine_type": {
		Default: storage.PostgresMachineTypeDefault.String(),
		Values:  stringerSlice(storage.PostgresMachineTypes),
	},
	"postgres_version": {
		Default: string(storage.PostgresVersionDefault),
		Values:  stringSlice(storage.PostgresVersions),
	},
	"postgres_keep_daily_backups": {Default: strconv.Itoa(storage.PostgresBackupRetentionDaysDefault)},

	// PostgreSQL databases
	"postgresdatabase_location": {
		Default: string(storage.PostgresDatabaseLocationDefault),
		Values:  stringSlice(storage.PostgresDatabaseLocationOptions),
	},
	"postgresdatabase_version": {
		Default: string(storage.PostgresDatabaseVersionDefault),
		Values:  stringSlice(storage.PostgresDatabaseVersions),
	},
	"postgresdatabase_backup_schedule": {
		Default: string(storage.DatabaseBackupScheduleCalendarDaily),
		Values:  stringSlice(storage.DatabaseBackupScheduleCalendars),
	},
	"postgresdatabase_collation": {Default: string(storage.PostgresDatabaseCollationDefault)},

	// MySQL
	"mysql_location": {
		Default: string(storage.MySQLLocationDefault),
		Values:  stringSlice(storage.MySQLLocationOptions),
	},
	"mysql_machine_type": {
		Default: storage.MySQLMachineTypeDefault.String(),
		Values:  stringerSlice(storage.MySQLMachineTypes),
	},
	"mysql_version": {
		Default: string(storage.MySQLVersionDefault),
		Values:  stringSlice(storage.MySQLVersions),
	},
	"mysql_character_set_name":      {Default: storage.MySQLCharsetDefault},
	"mysql_character_set_collation": {Default: storage.MySQLCollationDefault},
	"mysql_long_query_time":         {Default: string(storage.MySQLLongQueryTimeDefault)},
	"mysql_min_word_length":         {Default: strconv.Itoa(storage.MySQLMinWordLengthDefault)},
	"mysql_transaction_isolation":   {Default: string(storage.MySQLTransactionIsolationDefault)},
	"mysql_keep_daily_backups":      {Default: strconv.Itoa(storage.MySQLBackupRetentionDaysDefault)},

	// MySQL databases
	"mysqldatabase_location": {
		Default: string(storage.MySQLDatabaseLocationDefault),
		Values:  stringSlice(storage.MySQLDatabaseLocationOptions),
	},
	"mysqldatabase_version": {
		Default: string(storage.MySQLDatabaseVersionDefault),
		Values:  stringSlice(storage.MySQLDatabaseVersions),
	},
	"mysqldatabase_character_set": {
		Default: mysqlDatabaseCharacterSet,
		Values:  []string{mysqlDatabaseCharacterSet},
	},
	"mysqldatabase_backup_schedule": {
		Default: string(storage.DatabaseBackupScheduleCalendarDaily),
		Values:  stringSlice(storage.DatabaseBackupScheduleCalendars),
	},

	// OpenSearch
	"opensearch_location": {
		Default: string(storage.OpenSearchLocationDefault),
		Values:  stringSlice(storage.OpenSearchLocationOptions),
	},
	"opensearch_machine_type": {
		Default: storage.OpenSearchMachineTypeDefault.String(),
		Values:  stringerSlice(storage.OpenSearchMachineTypes),
	},
	"opensearch_cluster_type": {
		Default: string(storage.OpenSearchClusterTypeDefault),
		Values:  stringSlice(storage.OpenSearchClusterTypes),
	},
	"opensearch_version": {
		Default: string(storage.OpenSearchVersionDefault),
		Values:  stringSlice(storage.OpenSearchVersions),
	},

	// Key-Value Store
	"keyvaluestore_location": {
		Default: string(storage.KeyValueStoreLocationDefault),
		Values:  stringSlice(storage.KeyValueStoreLocationOptions),
	},
	"keyvaluestore_memory_size":       {Default: storage.KeyValueStoreMemorySizeDefault},
	"keyvaluestore_max_memory_policy": {Default: string(storage.KeyValueStoreMaxMemoryPolicyDefault)},

	// Buckets
	"bucket_location": {
		Default: string(storage.BucketUserLocationDefault),
		Values:  stringSlice(storage.BucketLocationOptions),
	},
	"bucketuser_location": {
		Default: string(storage.BucketUserLocationDefault),
		Values:  stringSlice(storage.BucketUserLocationOptions),
	},

	// CloudVMs
	"cloudvm_os": {Values: stringSlice(infrastructure.CloudVirtualMachineOperatingSystems)},
}

// Predictors returns completion predictors for all known API fields.
func Predictors() map[string]complete.Predictor {
	predictors := make(map[string]complete.Predictor, len(fields))
	for name, f := range fields {
		if len(f.Values) == 0 {
			predictors[qualify(name)] = complete.PredictAnything
			continue
		}
		predictors[qualify(name)] = complete.PredictSet(slices.Clone(f.Values)...)
	}

	return predictors
}

func stringSlice[K ~string](elems []K) []string {
	s := make([]string, 0, len(elems))
	for _, elem := range elems {
		s = append(s, string(elem))
	}
	return s
}

func stringerSlice[T fmt.Stringer](slice []T) []string {
	strings := make([]string, 0, len(slice))
	for _, e := range slice {
		strings = append(strings, e.String())
	}
	return strings
}
