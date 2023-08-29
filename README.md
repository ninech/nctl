# nctl

```bash
$ nctl --help
Usage: nctl <command>

Interact with Nine API resources. See https://docs.nineapis.ch for the full API docs.

Run "nctl <command> --help" for more information on a command.
```

## Setup

```bash
# If you have go already installed
go install github.com/ninech/nctl@latest

# Homebrew
brew install ninech/taps/nctl

# Debian/Ubuntu
echo "deb [trusted=yes] https://repo.nine.ch/deb/ /" > /etc/apt/sources.list.d/repo.nine.ch.list
apt update
apt install nctl

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
```

## Getting started

* login to the API using `nctl auth login`
* run `nctl --help` to get a list of all available commands

## Deplo.io Beta

Some commands in `nctl` interact with resources only available if you are part
of the [deplo.io](https://deplo.io) beta. If you are interested you can read
more about it and also sign up [here](https://deplo.io).
