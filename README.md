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
| `-server` | config / *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | config / `~/kubernetes` | Destination directory on the remote host |
| `-identity` | auto | SSH private key path |
| `-exclude` | | Additional exclude pattern, gitignore-style (repeatable) |
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
| `-server` | config / *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | config / `~/kubernetes` | Source directory on the remote host |
| `-identity` | auto | SSH private key path |

```sh
# Pull ~/kubernetes from remote into the current directory
gomvtossh pull -server deploy@myserver.example.com

# Pull into a specific local directory
gomvtossh pull -server deploy@myserver.example.com ./my-configs
```

## Configuration

Create `.gomvtossh.yml` in the project directory (or `~/.gomvtossh.yml` as a user default). Flags always override config values.

```yaml
server: deploy@myserver.example.com
remote-dir: ~/kubernetes

excludes:
  - "*.bak"
  - "*.tmp"
  - "dist/"
  - "node_modules/"
  - "**/vendor/"
```

The `excludes` list uses [gitignore](https://git-scm.com/docs/gitignore) syntax:

| Pattern | Matches |
|---------|---------|
| `*.log` | any file named `*.log` at any depth |
| `dist/` | any entry named `dist` at any depth |
| `/vendor` | `vendor` only at the root |
| `src/**/*.go` | `.go` files at any depth under `src/` |
| `!important.log` | negate a previous match (last rule wins) |
| `# comment` | ignored |

### Default excludes (always active during push)

- `.git`
- `sealed-secrets-master-key-backup.yaml`
- `gomvtossh` (the binary itself)

Config `excludes` and `-exclude` flags are added on top of these.

## Authentication

SSH key authentication only. When `-identity` is not set, keys are tried in order:
`~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, `~/.ssh/id_ecdsa`.

Passphrase-protected keys are supported via interactive prompt.
The remote host must already be present in `~/.ssh/known_hosts`.
