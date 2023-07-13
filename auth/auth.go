package auth

type Cmd struct {
	Login      LoginCmd      `cmd:"" help:"Login to nineapis.ch."`
	Cluster    ClusterCmd    `cmd:"" help:"Authenticate with Kubernetes Cluster."`
	OIDC       OIDCCmd       `cmd:"" help:"Perform interactive OIDC login." hidden:""`
	SetProject SetProjectCmd `cmd:"" help:"Set the default project to be used."`
	SetOrg     SetOrgCmd     `cmd:"" help:"Set the organization to be used."`
	Whoami     WhoAmICmd     `cmd:"" help:"Show who you are logged in as, your active organization and all your available organizations."`
}
