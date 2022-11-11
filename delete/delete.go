package delete

type Cmd struct {
	VCluster vclusterCmd `cmd:"" name:"vcluster" help:"Delete a vcluster."`
}
