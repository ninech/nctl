package get

type Cmd struct {
	Output        output      `help:"Configures list output. ${enum}" short:"o" enum:"full,no-header,contexts" default:"full"`
	AllNamespaces bool        `help:"apply the get over all namespaces." short:"A"`
	Clusters      clustersCmd `cmd:"" help:"Get Kubernetes Clusters."`
}

type output string

const (
	full     output = "full"
	noHeader output = "no-header"
	contexts output = "contexts"
)
