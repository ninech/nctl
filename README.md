# nctl

[![CI](https://github.com/ninech/nctl/actions/workflows/go.yml/badge.svg)](https://github.com/ninech/nctl/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/ninech/nctl)](https://github.com/ninech/nctl/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/ninech/nctl)](https://goreportcard.com/report/github.com/ninech/nctl)
[![License](https://img.shields.io/github/license/ninech/nctl)](LICENSE)

`nctl` is the command-line interface for [Nine](https://nine.ch)'s cloud platform.
It lets you manage applications, services, storage, and more from your terminal.

**[Documentation](https://docs.nine.ch/docs/nctl/)**

## Resources

| Category                   | Resources                                                          |
| -------------------------- | ------------------------------------------------------------------ |
| **deplo.io**               | Applications, Builds, Releases, Configs                            |
| **storage.nine.ch**        | PostgreSQL, MySQL, OpenSearch, KeyValueStore, Buckets, BucketUsers |
| **infrastructure.nine.ch** | Kubernetes Clusters, CloudVMs                                      |
| **networking.nine.ch**     | ServiceConnections                                                 |
| **iam.nine.ch**            | APIServiceAccounts                                                 |
| **management.nine.ch**     | Projects                                                           |

## Installation

```bash
# If you have go already installed
go install github.com/ninech/nctl@latest

# Homebrew
brew install ninech/taps/nctl

# Debian/Ubuntu
echo "deb [trusted=yes] https://repo.nine.ch/deb/ /" | sudo tee /etc/apt/sources.list.d/repo.nine.ch.list
sudo apt-get update
sudo apt-get install nctl

# Fedora/RHEL
cat <<EOF > /etc/yum.repos.d/repo.nine.ch.repo
[repo.nine.ch]
name=Nine Repo
baseurl=https://repo.nine.ch/yum/
enabled=1
gpgcheck=0
EOF
dnf install nctl

# Arch
# Install yay: https://github.com/Jguer/yay#binary
yay --version
yay -S nctl-bin

# EGet
# Install eget https://github.com/zyedidia/eget
eget ninech/nctl
```

Binaries for macOS, Linux and Windows can be found on the [releases](https://github.com/ninech/nctl/releases) page.

## Getting Started

1. Login to the API: `nctl auth login`
2. Explore available commands: `nctl --help`

For complete documentation, tutorials, and guides, visit **[docs.nine.ch/docs/nctl/](https://docs.nine.ch/docs/nctl/)**.

## Development

```bash
make           # Build nctl
make test      # Run tests
make lint      # Run linters
make lint-fix  # Run linters and fix issues
make update    # Update dependencies
make clean     # Remove built artifacts
```

## License

[Apache 2.0](LICENSE)
