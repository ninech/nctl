// Package exec provides the implementation for the exec command.
package exec

// Cmd holds all exec sub-commands.
type Cmd struct {
	Application      applicationCmd      `cmd:"" group:"deplo.io" aliases:"app,application" name:"application" help:"Execute a command or shell in a deplo.io application."`
	Postgres         postgresCmd         `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Connect to a PostgreSQL instance."`
	PostgresDatabase postgresDatabaseCmd `cmd:"" group:"storage.nine.ch" name:"postgresdatabase" help:"Connect to a PostgreSQL database."`
	MySQL            mysqlCmd            `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Connect to a MySQL instance."`
	MySQLDatabase    mysqlDatabaseCmd    `cmd:"" group:"storage.nine.ch" name:"mysqldatabase" help:"Connect to a MySQL database."`
	KeyValueStore    kvsCmd              `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Connect to a KeyValueStore instance."`
}

type resourceCmd struct {
	Name string `arg:"" completion-predictor:"resource_name" help:"Name of the resource." required:""`
}
