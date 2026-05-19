# gomvtossh

Syncs files and directories to/from a remote server over SSH — no temporary files on either side.

Designed for deploying Kubernetes manifests and similar config directories.

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

## Commands

```
gomvtossh <command> [flags] [args]
```

Run `gomvtossh help` for an overview, or `gomvtossh help <command>` for detailed flags.

---

### push

Uploads a local directory or single file to the remote host.

```
gomvtossh push [flags] [source] [remote-path]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | config / *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | config / `~/kubernetes` | Remote destination directory |
| `-identity` | auto | SSH private key path |
| `-exclude` | | Additional exclude pattern, gitignore-style (repeatable) |
| `-dry-run` | `false` | List files without transferring |

```sh
# Push current directory to ~/kubernetes
gomvtossh push -server deploy@host.example.com

# Push a specific directory with extra excludes
gomvtossh push -server deploy@host.example.com -exclude dist ./my-configs

# Push a single file to an exact remote path
gomvtossh push -server deploy@host.example.com ./deploy.yaml ~/kubernetes/deploy.yaml

# Preview what would be pushed
gomvtossh push -server deploy@host.example.com -dry-run
```

---

### pull

Downloads a remote directory or single file to the local machine. When a file already
exists locally, you are prompted:

```
  conflict  subdir/file.yaml
    [s]kip  [S]kip all  [o]verwrite  [O]verwrite all  [a]bort:
```

```
gomvtossh pull [flags] [remote-path local-path | local-dir]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | config / *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | config / `~/kubernetes` | Remote source directory |
| `-identity` | auto | SSH private key path |

```sh
# Pull ~/kubernetes into the current directory
gomvtossh pull -server deploy@host.example.com

# Pull into a specific local directory
gomvtossh pull -server deploy@host.example.com ./my-configs

# Pull a single remote file to a local path
gomvtossh pull -server deploy@host.example.com ~/kubernetes/deploy.yaml ./deploy.yaml
```

---

### ls

Lists files on the remote host.

```
gomvtossh ls [flags] [remote-path]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | config / *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | config / `~/kubernetes` | Default path to list |
| `-identity` | auto | SSH private key path |
| `-l` | `false` | Long listing format |
| `-a` | `false` | Include hidden entries |
| `-h` | `false` | Human-readable sizes (with `-l`) |
| `-R` | `false` | Recursive |

```sh
# List ~/kubernetes
gomvtossh ls -server deploy@host.example.com

# Long listing of a specific path
gomvtossh ls -server deploy@host.example.com -l ~/kubernetes/manifests
```

---

## Configuration

Create `.gomvtossh.yml` in the project directory (or `~/.gomvtossh.yml` as a user default).
Flags always override config values.

```yaml
server: deploy@host.example.com
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

Config `excludes` and `-exclude` flags are appended on top of these.

## Authentication

SSH key authentication only. When `-identity` is not set, keys are tried in order:
`~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, `~/.ssh/id_ecdsa`.

Passphrase-protected keys are supported via interactive prompt.
The remote host must already be present in `~/.ssh/known_hosts`.
