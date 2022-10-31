# nctl

```bash
$ nctl --help
Usage: nctl <command>

Interact with Nine API resources.

Flags:
  -h, --help                         Show context-sensitive help.
  -n, --namespace=STRING             Limit commands to a namespace.
      --api-cluster="nineapis.ch"    Context name of the API cluster.

Commands:
  get clusters
    Get Kubernetes Clusters.

  auth login
    Login to nineapis.ch.

  auth cluster <name>
    Authenticate with Kubernetes Cluster.

  completions
    Print shell completions.

Run "nctl <command> --help" for more information on a command.
```

## Getting started

* download the binary from the latest release or if you have go installed `go install github.com/ninech/nctl@latest`
* add `nctl` to your PATH
* login to the API using `nctl auth login <organization>`
