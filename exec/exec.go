package exec

type Cmd struct {
	Application applicationCmd `cmd:"" group:"deplo.io" aliases:"app" name:"application" help:"Execute a command or shell in a deplo.io application."`
}

type resourceCmd struct {
	Name string `arg:"" predictor:"resource_name" help:"Name of the application to exec command/shell in." required:""`
}
