# gomvtossh

Syncs a local directory to a remote server over SSH. Files are packed into a tar+gzip stream and piped directly into the remote `tar` command — no temporary files on either side.

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
gomvtossh [flags] [source-dir]
```

`source-dir` defaults to `.` (the current directory).

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | `~/kubernetes` | Destination directory on the remote host |
| `-identity` | auto | SSH private key path |
| `-exclude` | | Extra name to exclude (repeatable) |
| `-dry-run` | `false` | List files that would be synced, without transferring |

### Examples

```sh
# Sync current directory to ~/kubernetes on the remote host
gomvtossh -server deploy@myserver.example.com

# Sync a specific directory, skip extra patterns
gomvtossh -server deploy@myserver.example.com \
  -exclude dist -exclude node_modules \
  ./my-configs

# Preview what would be transferred
gomvtossh -server deploy@myserver.example.com -dry-run
```

## Default excludes

These names are always excluded, regardless of where they appear in the directory tree:

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
