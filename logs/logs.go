package logs

type Cmd struct {
	Applications applicationCmd `cmd:"" group:"deplo.io" name:"application" aliases:"app" help:"Get deplo.io Application logs. (Beta - requires access)"`
}
