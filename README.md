# gossh

Syncs files and directories to/from a remote server over SSH — no temporary files on either side.

Designed for deploying Kubernetes manifests and similar config directories.

## Installation

```sh
go install github.com/noobygames/gossh@latest
```

Or build from source:

```sh
git clone https://github.com/noobygames/gossh
cd gossh
go build -o gossh .
```

## Commands

```
gossh <command> [flags] [args]
```

Run `gossh help` for an overview, or `gossh help <command>` for detailed flags.

---

### push

Uploads a local directory or single file to the remote host.

```
gossh push [flags] [source] [remote-path]
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
gossh push -server deploy@host.example.com

# Push a specific directory with extra excludes
gossh push -server deploy@host.example.com -exclude dist ./my-configs

# Push a single file to an exact remote path
gossh push -server deploy@host.example.com ./deploy.yaml ~/kubernetes/deploy.yaml

# Preview what would be pushed
gossh push -server deploy@host.example.com -dry-run
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
gossh pull [flags] [remote-path local-path | local-dir]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | config / *(required)* | SSH target: `user@host` or `user@host:port` |
| `-remote-dir` | config / `~/kubernetes` | Remote source directory |
| `-identity` | auto | SSH private key path |

```sh
# Pull ~/kubernetes into the current directory
gossh pull -server deploy@host.example.com

# Pull into a specific local directory
gossh pull -server deploy@host.example.com ./my-configs

# Pull a single remote file to a local path
gossh pull -server deploy@host.example.com ~/kubernetes/deploy.yaml ./deploy.yaml
```

---

### ls

Lists files on the remote host.

```
gossh ls [flags] [remote-path]
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
gossh ls -server deploy@host.example.com

# Long listing of a specific path
gossh ls -server deploy@host.example.com -l ~/kubernetes/manifests
```

---

### kubectl

Runs `kubectl` on the remote host. Arguments that are local files or directories
are automatically uploaded to a temporary remote directory, substituted in the
command, and cleaned up after execution.

`-server` and `-identity` must come **before** the kubectl subcommand. Everything
after the first non-gossh argument is passed to kubectl unchanged.

```
gossh kubectl [-server host] [-identity key] <kubectl args...>
```

```sh
# Apply a local manifest
gossh kubectl apply -f ./manifest.yaml

# Apply an entire local directory
gossh kubectl apply -f ./manifests/

# Works with --filename= form too
gossh kubectl apply --filename=./deploy.yaml --dry-run=client

# Interactive commands get a PTY automatically
gossh kubectl exec -it mypod -- bash
```

---

### helm

Same auto-upload behaviour as `kubectl`, for Helm charts and values files.

```
gossh helm [-server host] [-identity key] <helm args...>
```

```sh
# Install a local chart
gossh helm install myrelease ./my-chart/

# Install with a local values file
gossh helm install myrelease ./my-chart/ -f ./values-prod.yaml

# Upgrade with --install
gossh helm upgrade myrelease ./my-chart/ --install
```

---

## Configuration

Create `.gossh.yml` in the project directory (or `~/.gossh.yml` as a user default).
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
- `gossh` (the binary itself)

Config `excludes` and `-exclude` flags are appended on top of these.

## Authentication

SSH key authentication only. When `-identity` is not set, keys are tried in order:
`~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, `~/.ssh/id_ecdsa`.

Passphrase-protected keys are supported via interactive prompt.
The remote host must already be present in `~/.ssh/known_hosts`.
