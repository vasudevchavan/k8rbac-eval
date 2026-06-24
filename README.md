# KubeAccess

`kubeaccess` is a CLI tool for inspecting Kubernetes RBAC access levels for users and service accounts, and for generating ready-to-apply Role/ClusterRole manifests. It includes a Carbon Design System / React web UI backed by a lightweight Go API server.

## Features

- **Check Access** — verify which verbs a user or service account has on any resource, namespace- or cluster-wide
- **Generate RBAC Manifests** — produce Role/RoleBinding or ClusterRole/ClusterRoleBinding YAML in one command
- **Impersonation** — uses Kubernetes impersonation so results reflect real effective permissions
- **Multi-platform detection** — detects EKS, AKS, OpenShift, and surfaces cloud-identity advisories (IRSA, Azure Workload Identity)
- **Web UI** — Carbon / React frontend with kubeconfig switcher, resource quick-picks, and live YAML output

## Prerequisites

- Go 1.21+ (for building from source)
- Node.js 18+ and npm (for the UI)
- A kubeconfig pointing at your cluster (`~/.kube/config` or `KUBECONFIG` env)
- Permission to impersonate users/groups on the cluster (`system:masters` or an equivalent impersonation ClusterRole)

---

## Installation

### From source (Make)

```bash
git clone https://github.com/vasudevchavan/k8rbac-eval.git
cd k8rbac-eval
make build            # → bin/kubeaccess
make build-server     # → bin/kubeaccess-server (optional, for the UI)
make build-all        # cross-compile for all platforms → bin/
```

### Using Go directly

```bash
go install github.com/vasudevchavan/k8s-get-access-level@latest
```

---

## CLI Usage

The CLI has two top-level commands: `show` and `generate`.

### Show Access

```bash
kubeaccess show user <username>  [flags]
kubeaccess show sa   <saname>    [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --namespace` | `default` | Target namespace |
| `--resource` | _(all)_ | Resource to check (e.g. `pods`, `deploy`). Omit to check every resource. |
| `-c, --clusterscope` | `false` | Check cluster-wide access instead of namespace-scoped |
| `--kubeconfig` | `KUBECONFIG` env → `~/.kube/config` | Path to a kubeconfig file |

### Generate Manifests

```bash
kubeaccess generate user <username>  --resource <resource> [flags]
kubeaccess generate sa   <saname>    --resource <resource> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--resource` | _(required)_ | Kubernetes resource (e.g. `pods`, `deployments`) |
| `--verb` | `get,list,watch` | Verbs to allow; repeatable (`--verb create --verb delete`) |
| `-n, --namespace` | `default` | Namespace for the Role/RoleBinding |
| `-c, --clusterscope` | `false` | Generate ClusterRole/ClusterRoleBinding instead |
| `--kubeconfig` | `KUBECONFIG` env → `~/.kube/config` | Path to a kubeconfig file |

### Examples

```bash
# Check if user alice can access pods in the default namespace
kubeaccess show user alice -n default --resource pods

# Check if service account my-app has access to secrets
kubeaccess show sa my-app -n default --resource secrets

# Check all cluster-scoped access for user alice
kubeaccess show user alice -c

# Generate a Role letting bob create and delete deployments
kubeaccess generate user bob --resource deployments --verb create --verb delete

# Generate a ClusterRole for a service account to view nodes
kubeaccess generate sa monitor-sa --resource nodes --verb get --verb list -c

# Target a different cluster
kubeaccess show user alice --resource pods --kubeconfig ~/.kube/staging.yaml
```

---

## Running the UI

The UI is a Carbon / React frontend backed by a Go HTTP API server that wraps the `kubeaccess` binary.

### Quickstart (one command)

```bash
make ui-start
# Builds bin/kubeaccess, starts the Go API server on :8080, and Vite on :3000
```

### Manual steps

**1. Build the CLI binary**

```bash
make build   # → bin/kubeaccess
```

**2. Start the API server**

The server auto-discovers the `kubeaccess` binary from `$PATH`, next to the server binary, or in `bin/`.

```bash
make run-server          # via Make
# or
go run ./cmd/server/main.go
```

**3. Start the React dev server**

```bash
cd ui && npm install && npm run dev
# Vite opens http://localhost:3000
# All /api/* requests are proxied to http://localhost:8080
```

**4. Production build**

```bash
make ui-build
# Output in ui/dist/ — serve as static files alongside the Go server
```

### Server environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Listen port for the API server |
| `KUBEACCESS_BIN` | auto-discovered | Explicit path to the `kubeaccess` binary |
| `CORS_ORIGIN` | `*` | Allowed CORS origin. Set to your UI's origin in production (e.g. `http://localhost:3000`) |

---

## Makefile targets

| Target | Description |
|--------|-------------|
| `make build` | Build CLI binary → `bin/kubeaccess` |
| `make build-server` | Build API server binary → `bin/kubeaccess-server` |
| `make build-all` | Cross-compile CLI for all platforms |
| `make run` | Run the CLI without building |
| `make run-server` | Run the API server without building |
| `make test` | Run all unit tests |
| `make test-e2e` | Run end-to-end tests (requires a live cluster) |
| `make fmt` | Format Go source |
| `make vet` | Run `go vet` |
| `make lint` | Run `golangci-lint` (must be installed separately) |
| `make ui-install` | Install Node dependencies |
| `make ui-build` | Production build of the React UI → `ui/dist/` |
| `make ui-dev` | Start UI with mock API (no cluster needed) |
| `make ui-start` | Build CLI + start API server + Vite dev server |
| `make clean` | Remove `bin/` and Go build cache |

---

## UI Features

### Kubeconfig Switcher

Every form has a kubeconfig selector that lists all kubeconfigs found in `~/.kube/` and the `KUBECONFIG` env var. Choose **"Custom path…"** to type any absolute path, or leave blank to use the default.

### Check Access tab

| Option | Description |
|--------|-------------|
| Subject type | **User** or **Service Account** |
| Name | Username or service account name |
| Namespace | Target namespace (disabled when cluster-scoped) |
| Resource | Quick-pick chips for common resources, or type a custom resource. Leave blank to check all. |
| Cluster-scoped | Toggle for cluster-wide check |

Results appear in a sortable table (verb → Allowed/Denied). A collapsible section shows raw CLI output.

### Generate RBAC tab

| Option | Description |
|--------|-------------|
| Subject type | **User** or **Service Account** |
| Name | Username or service account name |
| Namespace | Namespace for the Role/Binding |
| Resource | Required — quick-pick chips or custom input |
| Verbs | Checkboxes for `get`, `list`, `watch`, `create`, `update`, `patch`, `delete`, `deletecollection` |
| Cluster-scoped | Generate ClusterRole/ClusterRoleBinding instead |

Output is ready-to-apply YAML with **Copy** and **Download .yaml** buttons.

### Theme switcher

The header includes **Light** / **Dark** theme toggles.

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Liveness probe |
| `GET` | `/api/platform` | Detected cluster platform and flags |
| `GET` | `/api/kubeconfigs` | List available kubeconfig files |
| `POST` | `/api/check` | Run `kubeaccess show` |
| `POST` | `/api/generate` | Run `kubeaccess generate` |

### POST `/api/check`

```json
{
  "subjectType": "user",
  "name": "alice",
  "namespace": "default",
  "resource": "pods",
  "clusterScope": false,
  "kubeconfig": "/home/user/.kube/staging.yaml"
}
```

### POST `/api/generate`

```json
{
  "subjectType": "sa",
  "name": "monitor-sa",
  "namespace": "default",
  "resource": "nodes",
  "verbs": ["get", "list", "watch"],
  "clusterScope": true,
  "kubeconfig": ""
}
```
