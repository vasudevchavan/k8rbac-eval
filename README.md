# KubeAccess

`kubeaccess` is a CLI tool designed to inspect Kubernetes RBAC access levels for users and service accounts, and to generate RBAC manifests (Role/RoleBinding or ClusterRole/ClusterRoleBinding).

## Features

- **Check Access Level**: Verify if a specific user or service account has access to a resource in a namespace or cluster-wide.
- **Generate RBAC Manifests**: Automatically generate YAML manifests for Roles and Bindings based on desired access.
- **Impersonation Support**: Uses Kubernetes impersonation to accurately check effective permissions.
- **Web UI**: Carbon Design System / React frontend with a Go HTTP API server (see [Running the UI](#running-the-ui)).

## Prerequisites

- Go 1.20+ (for building from source)
- Node.js 18+ and npm (for the UI)
- A configured `~/.kube/config` file.
- **Permissions**: The user running this tool must have permissions to impersonate other users and groups (`system:masters` or similar privileges are often required for checking access of others).

---

## Installation

### From Source

Using Make:

```bash
git clone https://github.com/vasudevchavan/k8rbac-eval.git
cd k8rbac-eval
make build
# Binary will be in bin/kubeaccess

# Build for all platforms
make build-all
# Binaries will be in bin/ with suffixes (e.g., kubeaccess-linux-amd64)
```

Using Go directly:

```bash
go install github.com/vasudevchavan/k8s-get-access-level@latest
```

---

## CLI Usage

The CLI supports two main commands: `show` and `generate`.

### Show Access

Inspect the access level of a user or service account.

```bash
# Check user access for a resource in a namespace
kubeaccess show user <username> -n <namespace> --resource <resource>

# Check service account access
kubeaccess show sa <serviceaccount> -n <namespace> --resource <resource>
```

**Flags:**

- `-n, --namespace`: (Optional) Target namespace (default: `default`).
- `--resource`: (Optional) The Kubernetes resource to check (e.g., `pods`, `deployments`). Omit to check all resources.
- `-c, --clusterscope`: (Optional) Check cluster-level access.
- `--kubeconfig`: (Optional) Path to a kubeconfig file (defaults to `~/.kube/config`).

### Generate Manifests

Generate RBAC YAML manifests for a user or service account.

```bash
# Generate Role/Binding for a user
kubeaccess generate user <username> --resource <resource> --verb <verbs>

# Generate Role/Binding for a Service Account
kubeaccess generate sa <serviceaccount> --resource <resource> --verb <verbs>
```

**Flags:**

- `--verb`: (Optional) Verbs to include in the rule (default: `get`, `list`, `watch`). Can be repeated or comma-separated.
- `--resource`: (Required) Resource for the Role.
- `-n, --namespace`: (Optional) Namespace for the Role/Binding.
- `-c, --clusterscope`: (Optional) Generate a ClusterRole/ClusterRoleBinding instead.
- `--kubeconfig`: (Optional) Path to a kubeconfig file.

### Examples

```bash
# Check if user alice can access pods in default namespace
kubeaccess show user alice -n default --resource pods

# Check if service account my-app has secrets access
kubeaccess show sa my-app -n default --resource secrets

# Check all cluster-scoped access for user alice
kubeaccess show user alice -c

# Generate a Role allowing bob to create and delete deployments
kubeaccess generate user bob --resource deployments --verb create --verb delete

# Generate a ClusterRole for a service account to view nodes
kubeaccess generate sa monitor-sa --resource nodes --verb get --verb list -c

# Use a specific kubeconfig
kubeaccess show user alice --resource pods --kubeconfig ~/.kube/staging.yaml
```

---

## Running the UI

The UI is a Carbon Design System / React frontend backed by a lightweight Go HTTP API server that wraps the `kubeaccess` binary.

### 1. Build the kubeaccess binary

```bash
make build
# Produces bin/kubeaccess
```

### 2. Start the API server

The server auto-discovers the `kubeaccess` binary from `$PATH`, alongside the server binary, or in the project `bin/` directory.

```bash
go run ./cmd/server/main.go
# API listens on :8080 by default
```

Optional environment variables:

| Variable          | Default             | Description                              |
|-------------------|---------------------|------------------------------------------|
| `PORT`            | `8080`              | Port for the API server                  |
| `KUBEACCESS_BIN`  | auto-discovered     | Explicit path to the `kubeaccess` binary |

### 3. Start the React dev server

```bash
cd ui
npm install
npm run dev
# Opens http://localhost:3000
```

The Vite dev server proxies all `/api/*` requests to `http://localhost:8080`, so no CORS configuration is needed during development.

### 4. Production build

```bash
cd ui
npm run build
# Output in ui/dist/ — serve as static files alongside the Go server
```

---

## UI Options & Features

### Kubeconfig Switcher (top of every form)

- Automatically lists all kubeconfig files found in `~/.kube/` and the `KUBECONFIG` environment variable.
- Select any file from the dropdown to target a different cluster.
- Choose **"Custom path…"** or click **"Enter custom path instead"** to type any absolute path.
- Leave blank to use the default `~/.kube/config`.

### Check Access tab

| Option | Description |
|--------|-------------|
| Subject type | Toggle between **User** and **Service Account** |
| Name | Username or service account name |
| Namespace | Target namespace (disabled when cluster-scoped) |
| Resource | Quick-pick chip for common resources, or type any custom resource. Leave blank to check all resources. |
| Cluster-scoped | Toggle to run a cluster-wide check (`-c` flag) |

Results are displayed in a sortable table showing each verb (`get`, `list`, `watch`, etc.) and whether it is **Allowed** or **Denied**. Raw CLI output is available in a collapsible section.

### Generate RBAC tab

| Option | Description |
|--------|-------------|
| Subject type | Toggle between **User** and **Service Account** |
| Name | Username or service account name |
| Namespace | Namespace for the Role/Binding (disabled when cluster-scoped) |
| Resource | Quick-pick chips for common resources, or type any custom resource (required) |
| Verbs | Checkboxes for all standard Kubernetes verbs (`get`, `list`, `watch`, `create`, `update`, `patch`, `delete`, `deletecollection`) |
| Cluster-scoped | Toggle to generate a ClusterRole/ClusterRoleBinding instead of Role/RoleBinding |

Output is a ready-to-apply YAML manifest with **Copy** and **Download .yaml** buttons.

### Theme switcher

The header includes **Light** / **Dark** theme toggles.

---

## API Endpoints (Go server)

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/api/health` | Liveness probe |
| `GET`  | `/api/kubeconfigs` | List available kubeconfig files |
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
