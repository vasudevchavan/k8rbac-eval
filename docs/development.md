# KubeAccess — Development Guide

This guide covers local setup, testing, and how to extend KubeAccess.

---

## Prerequisites

| Tool | Minimum version | Purpose |
|------|----------------|---------|
| Go | 1.21 | Build CLI and API server |
| Node.js | 18 | Build/run the React UI |
| npm | 9 | UI dependency management |
| Docker | any | Run kind cluster for testing |
| kind | 0.20 | Local Kubernetes cluster |
| kubectl | any | Interact with the cluster |
| golangci-lint | 1.57 | Linting (optional) |

---

## Local setup

### 1. Clone and build

```bash
git clone https://github.com/vasudevchavan/k8rbac-eval.git
cd k8rbac-eval
make build          # → bin/kubeaccess
make build-server   # → bin/kubeaccess-server
```

### 2. Start a kind cluster

The project is tested against a [kind](https://kind.sigs.k8s.io/) (Kubernetes-in-Docker) cluster. This is the recommended local setup.

```bash
kind create cluster --name kubeaccess-dev
kubectl config use-context kind-kubeaccess-dev
```

Verify:

```bash
kubectl cluster-info --context kind-kubeaccess-dev
# Kubernetes control plane is running at https://127.0.0.1:<port>
```

> **Note:** kind's admin user is `kubernetes-admin` which belongs to `system:masters`. It has full cluster access — use a test service account (see [Manual smoke test](#manual-smoke-test-against-kind)) to exercise realistic permission checks.

### 3. Run the full stack

```bash
make ui-start
# Builds bin/kubeaccess, starts Go API on :8080, starts Vite UI on :3000
```

Open [http://localhost:3000](http://localhost:3000).

### 4. UI-only development (no cluster)

Use the mock API server — no Kubernetes needed:

```bash
make ui-dev
# Starts ui/mock-api.cjs on :8080 and Vite on :3000
```

---

## Testing

### Unit tests

```bash
make test
# or
go test ./...
```

Key test files:

| File | What it tests |
|------|--------------|
| `pkg/discovery/resources_test.go` | `GetAllResources`, `IsNamespaced` |
| `internal/cli/generate_test.go` | Manifest generation output |

### End-to-end tests

Requires a live kubeconfig (kind works):

```bash
make test-e2e
# Runs test/e2e.sh against whatever cluster kubectl points at
```

`test/e2e.sh` creates a test service account with a pod-reader RoleBinding (manifests in `test/serviceaccount/`), runs `kubeaccess show sa`, and asserts the expected output.

### Manual smoke test against kind

```bash
# Apply a test RoleBinding
kubectl apply -f test/serviceaccount/pod-reader-role.yaml
kubectl apply -f test/serviceaccount/pod-reader-rolebinding.yaml

# Check access for the test SA
./bin/kubeaccess show sa pod-reader-sa -n default --resource pods

# Expected output:
# resource: pods
#   get                : true
#   list               : true
#   watch              : true
#   create             : false
#   ...
```

---

## Code quality

```bash
make fmt        # go fmt ./...
make vet        # go vet ./...
make lint       # golangci-lint run ./... (requires golangci-lint in PATH)
```

Install golangci-lint:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

---

## Project layout quick reference

```
pkg/access/checker.go       — Kubernetes RBAC check logic (impersonation, SSAR, SSRR)
pkg/client/client.go        — kubeconfig loading and clientset construction
pkg/discovery/resources.go  — resource list via discovery API
pkg/discovery/scope.go      — namespaced vs cluster-scoped resolver
pkg/discovery/cache.go      — TTL cache for discovery results
pkg/generator/manifests.go  — Role/ClusterRole YAML generation
pkg/platform/detect.go      — cluster-type detection (OpenShift, EKS, AKS)
internal/cli/common.go      — RunAccessCheck: fast paths and worker pool
cmd/server/main.go          — HTTP API server
ui/src/components/          — React components (Carbon Design System)
ui/mock-api.cjs             — Standalone mock server for UI development
```

---

## How to extend

### Add a new resource to the UI default list

Edit `ui/src/components/CheckAccess.jsx`:

```jsx
const BASE_RESOURCES = [
  'pods', 'deployments', ...,
  'your-new-resource',   // ← add here
]
```

For platform-specific resources (e.g. an OpenShift resource):

```jsx
const PLATFORM_RESOURCES = {
  openshift: [
    'routes', ...,
    'your-openshift-resource',  // ← add here
  ],
  ...
}
```

### Add support for a new cloud platform

1. **Detection** (`pkg/platform/detect.go`): add a new `isXxx()` function and call it from `Detect()`. Add a new `Type` constant.

2. **User validation** (`cmd/server/main.go` → `validateUser()`): add a case for the new platform type to check whether the subject exists on that platform.

3. **Advisory messages** (`cmd/server/main.go` → `buildUserNotFoundMsg()`): add a platform-specific suggestion for what to do when a user isn't found.

4. **UI resources** (`ui/src/components/CheckAccess.jsx` → `PLATFORM_RESOURCES`): add platform-specific resource chips.

### Add a new CLI command

1. Create a new file in `internal/cli/` (e.g. `internal/cli/audit.go`).
2. Define a Cobra command and register it in `internal/cli/registercommands.go`.
3. Use `addCommonFlags()` to inherit the standard `--namespace`, `--resource`, `--clusterscope`, `--kubeconfig` flags.
4. Use `pkg/access.KubeChecker` for any RBAC queries.

### Change the set of verbs checked

Edit `pkg/access/checker.go`:

```go
var targetVerbs = []string{
    "get", "list", "watch", "create", "update", "patch", "delete",
    "deletecollection",  // ← add here
}
```

Also update `orderedVerbs` in `internal/cli/common.go` to control print order:

```go
var orderedVerbs = []string{
    "get", "list", "watch", "create", "update", "patch", "delete",
    "deletecollection",
}
```

---

## Environment variables (development)

| Variable | Default | Description |
|----------|---------|-------------|
| `KUBEACCESS_BIN` | auto-discovered | Override the binary path used by the API server |
| `PORT` | `8080` | API server listen port |
| `CORS_ORIGIN` | `*` | CORS allowed origin |
| `KUBECONFIG` | `~/.kube/config` | Standard kubeconfig env var |

---

## Debugging

### CLI verbose output

slog output goes to **stderr**, results go to **stdout**. Redirect them separately to inspect:

```bash
./bin/kubeaccess show user alice --resource pods 2>debug.log
cat debug.log   # slog lines
```

### API server request tracing

The server logs every request and its outcome via slog:

```bash
go run ./cmd/server/main.go 2>&1 | grep -E "level=INFO|level=WARN|level=ERROR"
```

### UI network tab

All API calls go to `/api/*` which Vite proxies to `:8080`. Open Chrome DevTools → Network → filter by `/api/` to inspect request/response payloads.

### Mock API customisation

Edit `ui/mock-api.cjs` to change the mock output. The `CHECK_OUTPUT` constant must match the CLI's stdout format exactly:

```
resource: <name>
  <verb>             : <true|false>
```

---

## Common issues

**`undefined: err` compilation error in cmd/server/main.go**
Each `exec.Command` block must use `err :=` (short declaration), not `err =` (assignment), because `err` is not declared in the outer scope of those blocks.

**`ECONNREFUSED` on `/api/kubeconfigs`**
The Vite dev server is running but the Go API server is not. Run `make ui-start` (not just `npm run dev`) to start both together.

**`SelfSubjectRulesReview incomplete`**
Your cluster uses a webhook authorizer (common with OPA/Gatekeeper). KubeAccess automatically falls back to the worker-pool path (individual `SelfSubjectAccessReview` per resource). Results are still accurate; they just take longer.

**User not found on kind**
kind uses `system:masters` for the admin user — no User objects exist. Use a service account for testing, or create a RoleBinding for a plain username and then check that username.

---

## Branching and commit conventions

| Branch pattern | Purpose |
|---------------|---------|
| `main` | Stable releases |
| `phase<N>` | Feature development phases |

Commit messages use the conventional format: `fix:`, `feat:`, `refactor:`, `docs:`, `test:`.
