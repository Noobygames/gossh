# gomvtossh

Syncs a local directory to/from a remote server over SSH using tar+gzip streams — no temporary files on either side.

Designed for deploying Kubernetes manifests and similar config directories where only the files themselves (not symlinks or special permissions) need to be transferred.

## Installation

```sh
go install github.com/noobygames/gomvtossh@latest
```

Or build from source:

```sh
git clone https://github.com/noobygames/gomvtossh
cd gomvtossh
go build -o gomvtossh .
```

## Usage

```
gomvtossh <push|pull> [flags] [dir]
```

`dir` defaults to `.` (the current directory).

### push

Transfers a local directory to the remote host.

```
gomvtossh push [flags] [source-dir]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | `~/kubernetes` | Destination directory on the remote host |
| `-identity` | auto | SSH private key path |
| `-exclude` | | Extra name to exclude (repeatable) |
| `-dry-run` | `false` | List files that would be transferred, without transferring |

```sh
# Push current directory to ~/kubernetes
gomvtossh push -server deploy@myserver.example.com

# Push a specific directory, skip extra patterns
gomvtossh push -server deploy@myserver.example.com \
  -exclude dist -exclude node_modules \
  ./my-configs

# Preview what would be pushed
gomvtossh push -server deploy@myserver.example.com -dry-run
```

### pull

Downloads files from the remote directory to a local directory. When a file already exists locally, you are prompted to choose:

```
  conflict  subdir/file.yaml
    [s]kip  [S]kip all  [o]verwrite  [O]verwrite all  [a]bort:
```

```
gomvtossh pull [flags] [local-dir]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | `~/kubernetes` | Source directory on the remote host |
| `-identity` | auto | SSH private key path |

```sh
# Pull ~/kubernetes from remote into the current directory
gomvtossh pull -server deploy@myserver.example.com

# Pull into a specific local directory
gomvtossh pull -server deploy@myserver.example.com ./my-configs
```

## Default push excludes

These names are always excluded during push, regardless of depth in the tree:

- `.git`
- `secret.yml`
- `sealed-secrets-master-key-backup.yaml`
- `gomvtossh` (the binary itself)

Additional names can be excluded with `-exclude`.

## Authentication

SSH key authentication only. When `-identity` is not set, keys are tried in order:
`~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, `~/.ssh/id_ecdsa`.

Passphrase-protected keys are supported via interactive prompt.
The remote host must already be present in `~/.ssh/known_hosts`.
