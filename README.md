# nctl

```bash
$ nctl --help
Usage: nctl <command>

Interact with Nine API resources.

Flags:
  -h, --help                         Show context-sensitive help.
  -n, --namespace=STRING             Limit commands to a namespace.
      --api-cluster="nineapis.ch"    Context name of the API cluster.
      --version                      Print version information and quit.

Commands:
  get clusters
    Get Kubernetes Clusters.

  get apiserviceaccounts (asa)
    Get API Service Accounts

  auth login <organization>
    Login to nineapis.ch.

  auth cluster <name>
    Authenticate with Kubernetes Cluster.

  completions
    Print shell completions.

  create vcluster [<name>]
    Create a new vcluster.

  create apiserviceaccount (asa) [<name>]
    Create a new API Service Account.

  delete vcluster <name>
    Delete a vcluster.

  delete apiserviceaccount (asa) <name>
    Delete a new API Service Account.

Run "nctl <command> --help" for more information on a command.
```

## Getting started

* download the binary from the latest release or if you have go installed `go install github.com/ninech/nctl@latest`
* add `nctl` to your PATH
* login to the API using `nctl auth login <organization>`
