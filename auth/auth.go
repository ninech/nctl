package auth

type Cmd struct {
	Login      LoginCmd      `cmd:"" help:"Login to nineapis.ch."`
	Cluster    ClusterCmd    `cmd:"" help:"Authenticate with Kubernetes Cluster."`
	OIDC       OIDCCmd       `cmd:"" help:"Perform interactive OIDC login." hidden:""`
	SetProject SetProjectCmd `cmd:"" help:"Set the default project to be used."`
}
